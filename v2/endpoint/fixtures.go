package endpoint

// Test fixture types used by executor_test.go and benchmark_test.go.

// CreateUserReq is a request type for testing JSON binding + validation.
type CreateUserReq struct {
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"gte=0,lte=150"`
}

// CreateUserResp is the response type for the create-user endpoint.
type CreateUserResp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PingReq is a request type with no fields, for testing no-binding endpoints.
type PingReq struct{}

// PingResp is the response type for the ping endpoint.
type PingResp struct {
	Message string `json:"message"`
}

// QueryReq is a request type for testing query binding.
type QueryReq struct {
	Page int `query:"page" validate:"gte=1"`
	Size int `query:"size" validate:"gte=1,lte=100"`
}

// QueryResp is the response type for query endpoints.
type QueryResp struct {
	Items []string `json:"items"`
	Total int      `json:"total"`
}

// problemReq is a request type for testing handler-returned problems.
type problemReq struct {
	Mode string `json:"mode" validate:"required,oneof=ok notfound conflict"`
}

// problemResp is the response type for problem endpoints.
type problemResp struct {
	Result string `json:"result"`
}
