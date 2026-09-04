package oss

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestNewClientConfigPathStyleByVendor 寻址风格按 endpoint 的厂商标识推导：
// 公有云走虚拟主机，自建（MinIO 等）走路径。用错风格会直接连不上，值得钉死。
func TestNewClientConfigPathStyleByVendor(t *testing.T) {
	cases := []struct {
		endpoint      string
		wantPathStyle bool
	}{
		{"127.0.0.1:9000", true},                    // MinIO
		{"minio.internal:9000", true},               // 自建
		{"oss-cn-beijing.aliyuncs.com", false},      // 阿里云
		{"cos.ap-beijing.myqcloud.com", false},      // 腾讯云
		{"s3-cn-north-1.qiniucs.com", false},        // 七牛
		{"obs.cn-north-4.myhuaweicloud.com", false}, // 华为云
		{"OSS-CN-BEIJING.ALIYUNCS.COM", false},      // 大小写不敏感
	}
	for _, c := range cases {
		got := NewClientConfig(Properties{Endpoint: c.endpoint}).UsePathStyle
		if got != c.wantPathStyle {
			t.Errorf("endpoint=%q UsePathStyle = %v, want %v", c.endpoint, got, c.wantPathStyle)
		}
	}
}

// TestNewClientConfigDefaults region/accessPolicy 留空时的兜底，以及 isHttps 只认 "Y"。
func TestNewClientConfigDefaults(t *testing.T) {
	cfg := NewClientConfig(Properties{Endpoint: "127.0.0.1:9000"})
	if cfg.Region != defaultRegion {
		t.Errorf("Region = %q, want %q", cfg.Region, defaultRegion)
	}
	if cfg.AccessPolicy != AccessPolicyPublicReadWrite {
		t.Errorf("AccessPolicy = %q, want %q", cfg.AccessPolicy, AccessPolicyPublicReadWrite)
	}
	if cfg.UseHTTPS {
		t.Error("isHttps 未配置时 UseHTTPS 应为 false")
	}
	// 库里 is_https 的合法值是 Y/N，"1"/"true" 都不该被当成开启。
	for _, v := range []string{"N", "1", "true", "y"} {
		if NewClientConfig(Properties{IsHttps: v}).UseHTTPS {
			t.Errorf("isHttps=%q 不应开启 https", v)
		}
	}
	if !NewClientConfig(Properties{IsHttps: "Y"}).UseHTTPS {
		t.Error("isHttps=Y 应开启 https")
	}
}

// TestIsPrivate 仅 accessPolicy=0 是私有桶——只有这一种才把 URL 换成预签名链接。
func TestIsPrivate(t *testing.T) {
	for policy, want := range map[string]bool{
		AccessPolicyPrivate:         true,
		AccessPolicyPublicReadWrite: false,
		AccessPolicyPublicRead:      false,
	} {
		got := NewClientConfig(Properties{AccessPolicy: policy}).IsPrivate()
		if got != want {
			t.Errorf("accessPolicy=%q IsPrivate() = %v, want %v", policy, got, want)
		}
	}
}

// TestBucketURL 桶地址的两种寻址风格，以及 domainUrl 覆盖 endpoint。
// 这个串直接落库成 sys_oss.url，拼错等于所有文件都访问不到。
func TestBucketURL(t *testing.T) {
	cases := []struct {
		name string
		p    Properties
		want string
	}{
		{
			name: "路径风格",
			p:    Properties{Endpoint: "127.0.0.1:9000", BucketName: "ruoyi"},
			want: "http://127.0.0.1:9000/ruoyi",
		},
		{
			name: "路径风格 + https",
			p:    Properties{Endpoint: "minio.internal", BucketName: "ruoyi", IsHttps: "Y"},
			want: "https://minio.internal/ruoyi",
		},
		{
			name: "虚拟主机风格",
			p:    Properties{Endpoint: "oss-cn-beijing.aliyuncs.com", BucketName: "ruoyi", IsHttps: "Y"},
			want: "https://ruoyi.oss-cn-beijing.aliyuncs.com",
		},
		{
			name: "domainUrl 覆盖 endpoint",
			p:    Properties{Endpoint: "127.0.0.1:9000", DomainURL: "cdn.example.com", BucketName: "ruoyi"},
			want: "http://cdn.example.com/ruoyi",
		},
		{
			// 库里 seed 的 endpoint 是裸 host:port，但用户可能带上协议头；
			// 不剥就会拼出 http://http://... 这种废地址。
			name: "endpoint 自带协议头时剥掉再按 isHttps 重加",
			p:    Properties{Endpoint: "https://127.0.0.1:9000", BucketName: "ruoyi"},
			want: "http://127.0.0.1:9000/ruoyi",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NewClientConfig(c.p).BucketURL(); got != c.want {
				t.Errorf("BucketURL() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestDomainURLFallback 未配 domainUrl 时预签名 endpoint 回落到服务地址。
func TestDomainURLFallback(t *testing.T) {
	cfg := NewClientConfig(Properties{Endpoint: "127.0.0.1:9000"})
	if got, want := cfg.DomainURL(), "http://127.0.0.1:9000"; got != want {
		t.Errorf("DomainURL() = %q, want %q", got, want)
	}

	cfg = NewClientConfig(Properties{Endpoint: "127.0.0.1:9000", DomainURL: "cdn.example.com"})
	if got, want := cfg.DomainURL(), "http://cdn.example.com"; got != want {
		t.Errorf("DomainURL() = %q, want %q", got, want)
	}
}

// pathKeyPattern 对象键的格式：[prefix/]yyyy/MM/dd/<32位十六进制>[.后缀]
var pathKeyPattern = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}/[0-9a-f]{32}`)

// TestBuildPathKey 对象键格式须逐字稳定——键直接落库，
// 日期段或 uuid 长度一变，按前缀扫桶的运维脚本就全失效。
func TestBuildPathKey(t *testing.T) {
	c := &Client{cfg: NewClientConfig(Properties{Endpoint: "127.0.0.1:9000"})}

	key := c.BuildPathKey("photo.png")
	if !pathKeyPattern.MatchString(key) {
		t.Errorf("key = %q 不符合 yyyy/MM/dd/<32位hex> 格式", key)
	}
	if !strings.HasSuffix(key, ".png") {
		t.Errorf("key = %q 应保留含点的后缀", key)
	}
	if want := time.Now().Format("2006/01/02"); !strings.HasPrefix(key, want) {
		t.Errorf("key = %q 应以当天日期 %s 开头", key, want)
	}

	// 无扩展名时后缀为空串（不复刻原项目造出假后缀的 bug）。
	if key := c.BuildPathKey("README"); lastSegmentExt(key) != "" {
		t.Errorf("无扩展名的文件 key = %q 不应有后缀", key)
	}

	// 多次调用不能重复，否则并发上传会互相覆盖。
	if a, b := c.BuildPathKey("a.txt"), c.BuildPathKey("a.txt"); a == b {
		t.Error("同名文件两次 BuildPathKey 不应得到相同的键")
	}
}

// TestBuildPathKeyWithPrefix 配了 prefix 时键带前缀段，且不拼出重复斜杠。
func TestBuildPathKeyWithPrefix(t *testing.T) {
	for _, prefix := range []string{"image", "/image", "image/", " /image/ "} {
		c := &Client{cfg: NewClientConfig(Properties{
			Endpoint: "127.0.0.1:9000",
			Prefix:   prefix,
		})}
		key := c.BuildPathKey("a.png")
		if !strings.HasPrefix(key, "image/") {
			t.Errorf("prefix=%q 生成的 key = %q 应以 image/ 开头", prefix, key)
		}
		if strings.Contains(key, "//") {
			t.Errorf("prefix=%q 生成的 key = %q 含重复斜杠", prefix, key)
		}
		if !pathKeyPattern.MatchString(strings.TrimPrefix(key, "image/")) {
			t.Errorf("prefix=%q 去掉前缀后的 key = %q 格式不对", prefix, key)
		}
	}
}

// lastSegmentExt 取 key 最后一段的扩展名，避免被路径里的点干扰。
func lastSegmentExt(key string) string {
	parts := strings.Split(key, "/")
	last := parts[len(parts)-1]
	if i := strings.LastIndex(last, "."); i >= 0 {
		return last[i:]
	}
	return ""
}
