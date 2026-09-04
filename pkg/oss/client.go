package oss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// PutObjectResult 上传结果。
type PutObjectResult struct {
	// URL 文件访问地址，= 桶地址 + "/" + Key，落 sys_oss.url。
	URL string
	// Key 对象键，落 sys_oss.file_name。
	Key  string
	ETag string
	Size int64
}

// GetObjectResult 下载对象的元信息，不含数据流。
type GetObjectResult struct {
	Key         string
	Size        int64
	ContentType string
	ETag        string
}

// Client 单个对象存储配置对应的 S3 客户端。
//
// 由 Instance/InstanceDefault 按 configKey 取用，不要自行构造：
// 工厂负责按配置变化重建，绕过它拿到的客户端不会随配置更新。
type Client struct {
	configKey string
	cfg       ClientConfig
	s3        *s3.Client
	presign   *s3.PresignClient
}

// newClient 按配置建 S3 客户端。
func newClient(configKey string, cfg ClientConfig) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("oss: endpoint 未配置")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("oss: bucketName 未配置")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("oss: accessKey/secretKey 未配置")
	}

	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	// 两个 checksum 开关必须显式设成 WhenRequired：默认值会给每个请求附带
	// x-amz-checksum-* 头与 trailer，MinIO / 阿里云等 S3 兼容实现不认，直接报错。
	base := s3.New(s3.Options{
		Credentials:                creds,
		Region:                     cfg.Region,
		BaseEndpoint:               aws.String(cfg.EndpointURL()),
		UsePathStyle:               cfg.UsePathStyle,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})

	// 预签名客户端的 endpoint 用 domainUrl 而非 endpointUrl：
	// 配了 CDN 域名时，签出的链接得指向 CDN，指回源站等于绕过 CDN。
	// 未配 domainUrl 时 DomainURL 回落 endpoint。
	presignBase := s3.New(s3.Options{
		Credentials:                creds,
		Region:                     cfg.Region,
		BaseEndpoint:               aws.String(cfg.DomainURL()),
		UsePathStyle:               cfg.UsePathStyle,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})

	return &Client{
		configKey: configKey,
		cfg:       cfg,
		s3:        base,
		presign:   s3.NewPresignClient(presignBase),
	}, nil
}

// ConfigKey 本客户端所用配置的 config_key，落 sys_oss.service。
func (c *Client) ConfigKey() string { return c.configKey }

// Config 返回本客户端的配置副本。
func (c *Client) Config() ClientConfig { return c.cfg }

// IsPrivate 是否私有桶。
func (c *Client) IsPrivate() bool { return c.cfg.IsPrivate() }

// BuildPathKey 生成对象键：[prefix/]yyyy/MM/dd/<32位十六进制>.<后缀>。
//
// 格式须固定——键直接落库，改了格式老数据仍按旧键存放，只是新旧混排；
// 但若日期段或 uuid 长度变了，任何按前缀扫描桶的运维脚本都会失效。
func (c *Client) BuildPathKey(fileName string) string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	datePath := time.Now().Format("2006/01/02")

	key := datePath + "/" + id + suffixOf(fileName)
	if prefix := normalizePrefix(c.cfg.Prefix); prefix != "" {
		key = prefix + "/" + key
	}
	return key
}

// suffixOf 取含点的后缀，无点返回空串。
//
// 不照搬 Java：那边用 lastIndexOf(".") 的 -1 结果直接喂给 hutool 的 substring，
// 被当成"倒数第一个字符"，无扩展名的文件会得到一个由末字符构成的假后缀。
func suffixOf(fileName string) string {
	return path.Ext(fileName)
}

// normalizePrefix 去掉前后空白与首尾斜杠，避免拼出 //。
func normalizePrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

// Upload 上传对象。size 为负表示长度未知，此时由 SDK 自行分片。
func (c *Client) Upload(ctx context.Context, key string, body io.Reader,
	size int64, contentType string) (*PutObjectResult, error) {

	in := &s3.PutObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}

	out, err := c.s3.PutObject(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("oss: 上传 %s 失败: %w", key, err)
	}

	res := &PutObjectResult{URL: c.cfg.BucketURL() + "/" + key, Key: key, Size: size}
	if out.ETag != nil {
		res.ETag = *out.ETag
	}
	if res.Size < 0 && out.Size != nil {
		res.Size = *out.Size
	}
	return res, nil
}

// Download 取对象元信息与数据流。调用方须关闭返回的流。
//
// 返回流而非字节切片：下载走 io.Copy 直通响应，大文件不会把进程内存打爆。
func (c *Client) Download(ctx context.Context, key string) (*GetObjectResult, io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("oss: 下载 %s 失败: %w", key, err)
	}

	res := &GetObjectResult{Key: key}
	if out.ContentLength != nil {
		res.Size = *out.ContentLength
	}
	if out.ContentType != nil {
		res.ContentType = *out.ContentType
	}
	if out.ETag != nil {
		res.ETag = *out.ETag
	}
	return res, out.Body, nil
}

// Delete 删除对象。
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("oss: 删除 %s 失败: %w", key, err)
	}
	return nil
}

// PresignGetURL 生成限时的预签名下载链接，供私有桶对外暴露文件。
func (c *Client) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("oss: 生成 %s 的预签名地址失败: %w", key, err)
	}
	return req.URL, nil
}
