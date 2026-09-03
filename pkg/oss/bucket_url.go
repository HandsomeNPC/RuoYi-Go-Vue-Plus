package oss

import "strings"

// 移植 Java BucketUrlUtil：按寻址风格拼桶地址。
//
// 配置里的 endpoint/domain 允许带或不带协议头（库里 seed 的是裸 host:port），
// 故统一先剥再按 isHttps 重加，避免出现 https://http://host 这种拼法。

// rebuildURLHeader 剥掉已有协议头后按 isHttps 重新加上。
func rebuildURLHeader(isHTTPS bool, base string) string {
	scheme := "http://"
	if isHTTPS {
		scheme = "https://"
	}
	return scheme + removeProtocolHeader(base)
}

// removeProtocolHeader 去掉 http:// 或 https:// 前缀，大小写不敏感。
func removeProtocolHeader(url string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(url) >= len(prefix) && strings.EqualFold(url[:len(prefix)], prefix) {
			return url[len(prefix):]
		}
	}
	return url
}

// pathStyleBucketURL 路径寻址：scheme://endpoint/bucket。MinIO 等自建服务用这种。
func pathStyleBucketURL(isHTTPS bool, base, bucket string) string {
	return rebuildURLHeader(isHTTPS, base) + "/" + bucket
}

// siteStyleBucketURL 虚拟主机寻址：scheme://bucket.endpoint。公有云用这种。
func siteStyleBucketURL(isHTTPS bool, base, bucket string) string {
	return rebuildURLHeader(isHTTPS, bucket+"."+removeProtocolHeader(base))
}
