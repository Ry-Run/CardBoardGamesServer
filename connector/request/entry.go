package request

type EntryReq struct {
	Token    string   `json:"token"`
	UserInfo UserInfo `json:"userInfo"`
}

type UserInfo struct {
	NickName string `json:"nickName"`
	Avatar   string `json:"avatar"`
	Sex      int    `json:"sex"`
}
