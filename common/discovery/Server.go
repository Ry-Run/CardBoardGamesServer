package discovery

import "fmt"

type Server struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Weight  int    `json:"weight"`
	Version string `json:"version"`
	Ttl     int64  `json:"ttl"`
}

// BuildRegisterKey
//
//		@Description: 构建放入 etcd 数据的 key
//		有 Version 则返回 /app/version/addr 形式：user/v1/10.0.0.1:110
//	 否则返回 /app/addr 形式：user/10.0.0.1:110
//		@s Server
//		@return key
func (s Server) BuildRegisterKey() string {
	if len(s.Version) == 0 {
		// /app/addr 形式：user/10.0.0.1:110
		return fmt.Sprintf("/%s/%s", s.Name, s.Addr)
	}
	// /app/version/addr 形式：user/v1/10.0.0.1:110
	return fmt.Sprintf("/%s/%s/%s", s.Name, s.Version, s.Addr)
}
