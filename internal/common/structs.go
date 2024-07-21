package common

type DataStruct struct {
	Error    bool   `json:"error"`
	ErrorMsg string `json:"error-message,omitempty"`
	Data     string `json:"data-base64,omitempty"`
}
