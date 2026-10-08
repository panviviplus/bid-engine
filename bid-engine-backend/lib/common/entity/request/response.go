package request

type JSONResponse interface {
	SetCode(code int)
	SetMessage(message string)
	SetRequestID(requestID string)
	SetData(data interface{})

	GetCode() int
	GetMessage() string
	GetRequestID() string
	GetData() interface{}
}

type jsonResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message" default:"ok"`
	RequestID string      `json:"requestId"`
	Data      interface{} `json:"data,omitempty"`
}

func (r *jsonResponse) SetCode(code int) {
	r.Code = code
}

func (r *jsonResponse) SetMessage(message string) {
	r.Message = message
}

func (r *jsonResponse) SetRequestID(requestID string) {
	r.RequestID = requestID
}

func (r *jsonResponse) SetData(data interface{}) {
	r.Data = data
}

func (r *jsonResponse) GetCode() int {
	return r.Code
}

func (r *jsonResponse) GetMessage() string {
	return r.Message
}

func (r *jsonResponse) GetRequestID() string {
	return r.RequestID
}

func (r *jsonResponse) GetData() interface{} {
	return r.Data
}

func NewJSONResponse() JSONResponse {
	return &jsonResponse{}
}

func NewJSONResponseOK(data interface{}) JSONResponse {
	return &jsonResponse{Code: 0, Message: "ok", Data: data}
}

func NewJSONResponseWithParams(code int, message, requestID string, data interface{}) JSONResponse {
	return &jsonResponse{Code: code, Message: message, RequestID: requestID, Data: data}
}
