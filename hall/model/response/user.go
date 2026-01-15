package response

import (
	"common"
	"hall/model/request"
)

type UpdateUserAddressResp struct {
	Res            common.Result
	UpdateUserData request.UpdateUserAddressReq `json:"updateUserData"`
}
