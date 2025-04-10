package client

import (
	"encoding/json"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
)

var (
	templateTypeMapping = map[TemplateType]TemplateType{
		TemplateTypeVerification: TemplateTypeNotification,
		TemplateTypeMarketing:    TemplateTypeMarketing,
		TemplateTypeNotification: TemplateTypeVerification,
	}
	_ Client = (*AliyunSMS)(nil)
)

// AliyunSMS 阿里云短信实现
type AliyunSMS struct {
	client *dysmsapi.Client
}

// NewAliyunSMS 创建阿里云短信实例
func NewAliyunSMS(regionID, accessKeyID, accessKeySecret string) (*AliyunSMS, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		RegionId:        tea.String(regionID),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}

	client, err := dysmsapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &AliyunSMS{client: client}, nil
}

func (a *AliyunSMS) CreateTemplate(req CreateTemplateReq) (CreateTemplateResp, error) {
	templateType, ok := templateTypeMapping[req.TemplateType]
	if !ok {
		return CreateTemplateResp{}, fmt.Errorf("%w: 模版类型非法", ErrInvalidParameter)
	}
	request := &dysmsapi.AddSmsTemplateRequest{
		TemplateType:    tea.Int32(int32(templateType)),
		TemplateName:    tea.String(req.TemplateName),
		TemplateContent: tea.String(req.TemplateContent),
		Remark:          tea.String(req.Remark),
	}

	response, err := a.client.AddSmsTemplate(request)
	if err != nil {
		return CreateTemplateResp{}, fmt.Errorf("%w: %w", ErrCreateTemplateFailed, err)
	}

	if response.Body == nil || response.Body.Code == nil || *response.Body.Code != OK {
		return CreateTemplateResp{}, fmt.Errorf("%w: %v", ErrCreateTemplateFailed, "响应异常")
	}

	return CreateTemplateResp{
		TemplateID: *response.Body.TemplateCode,
		RequestID:  *response.Body.RequestId,
	}, nil
}

func (a *AliyunSMS) QueryTemplateStatus(req QueryTemplateStatusReq) (QueryTemplateStatusResp, error) {
	request := &dysmsapi.QuerySmsTemplateRequest{
		TemplateCode: tea.String(req.TemplateID),
	}

	response, err := a.client.QuerySmsTemplate(request)
	if err != nil {
		return QueryTemplateStatusResp{}, fmt.Errorf("%w: %w", ErrQueryTemplateStatus, err)
	}

	if response.Body == nil || response.Body.Code == nil || *response.Body.Code != OK {
		return QueryTemplateStatusResp{}, fmt.Errorf("%w: %v", ErrQueryTemplateStatus, "响应异常")
	}

	var status int
	if response.Body.TemplateStatus != nil {
		status = int(*response.Body.TemplateStatus)
	}

	return QueryTemplateStatusResp{
		// 需要检查映射关系
		AuditStatus: AuditStatus(status),
		TemplateID:  req.TemplateID,
		RequestID:   *response.Body.RequestId,
		Reason:      *response.Body.Reason,
	}, nil
}

func (a *AliyunSMS) Send(req SendReq) (SendResp, error) {
	if len(req.PhoneNumbers) == 0 {
		return SendResp{}, fmt.Errorf("%w: %v", ErrInvalidParameter, "手机号码不能为空")
	}

	// 将多个手机号码用逗号分隔
	phoneNumbers := ""
	for i, phone := range req.PhoneNumbers {
		if i > 0 {
			phoneNumbers += ","
		}
		phoneNumbers += phone
	}

	templateParam := ""
	if req.TemplateParam != nil {
		jsonParams, err := json.Marshal(req.TemplateParam)
		if err != nil {
			return SendResp{}, fmt.Errorf("%w: %w", ErrInvalidParameter, err)
		}
		templateParam = string(jsonParams)
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phoneNumbers),
		SignName:      tea.String(req.SignName),
		TemplateCode:  tea.String(req.TemplateID),
		TemplateParam: tea.String(templateParam),
	}

	response, err := a.client.SendSms(request)
	if err != nil {
		return SendResp{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	if response.Body == nil || response.Body.Code == nil || *response.Body.Code != OK {
		return SendResp{}, fmt.Errorf("%w: %v", ErrSendFailed, "响应异常")
	}

	// 构建新的响应格式
	result := SendResp{
		RequestID:    *response.Body.RequestId,
		PhoneNumbers: make(map[string]SendRespStatus),
	}

	// 阿里云短信发送接口不返回每个手机号的状态，只返回整体状态
	// 所以这里为每个手机号设置相同的状态
	for _, phone := range req.PhoneNumbers {
		// 去掉可能的+86前缀
		cleanPhone := strings.TrimPrefix(phone, "+86")
		result.PhoneNumbers[cleanPhone] = SendRespStatus{
			Code:    *response.Body.Code,
			Message: *response.Body.Message,
		}
	}
	return result, nil
}
