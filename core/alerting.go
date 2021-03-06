package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/TwinProduction/gatus/client"
	"net/http"
	"net/url"
	"strings"
)

type AlertingConfig struct {
	Slack  string               `yaml:"slack"`
	Twilio *TwilioAlertProvider `yaml:"twilio"`
	Custom *CustomAlertProvider `yaml:"custom"`
}

type TwilioAlertProvider struct {
	SID   string `yaml:"sid"`
	Token string `yaml:"token"`
	From  string `yaml:"from"`
	To    string `yaml:"to"`
}

func (provider *TwilioAlertProvider) IsValid() bool {
	return len(provider.Token) > 0 && len(provider.SID) > 0 && len(provider.From) > 0 && len(provider.To) > 0
}

type CustomAlertProvider struct {
	Url     string            `yaml:"url"`
	Method  string            `yaml:"method,omitempty"`
	Body    string            `yaml:"body,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

func (provider *CustomAlertProvider) IsValid() bool {
	return len(provider.Url) > 0
}

func (provider *CustomAlertProvider) buildRequest(serviceName, alertDescription string, resolved bool) *http.Request {
	body := provider.Body
	providerUrl := provider.Url
	if strings.Contains(body, "[ALERT_DESCRIPTION]") {
		body = strings.ReplaceAll(body, "[ALERT_DESCRIPTION]", alertDescription)
	}
	if strings.Contains(body, "[SERVICE_NAME]") {
		body = strings.ReplaceAll(body, "[SERVICE_NAME]", serviceName)
	}
	if strings.Contains(body, "[ALERT_TRIGGERED_OR_RESOLVED]") {
		if resolved {
			body = strings.ReplaceAll(body, "[ALERT_TRIGGERED_OR_RESOLVED]", "RESOLVED")
		} else {
			body = strings.ReplaceAll(body, "[ALERT_TRIGGERED_OR_RESOLVED]", "TRIGGERED")
		}
	}
	if strings.Contains(providerUrl, "[ALERT_DESCRIPTION]") {
		providerUrl = strings.ReplaceAll(providerUrl, "[ALERT_DESCRIPTION]", alertDescription)
	}
	if strings.Contains(providerUrl, "[SERVICE_NAME]") {
		providerUrl = strings.ReplaceAll(providerUrl, "[SERVICE_NAME]", serviceName)
	}
	if strings.Contains(providerUrl, "[ALERT_TRIGGERED_OR_RESOLVED]") {
		if resolved {
			providerUrl = strings.ReplaceAll(providerUrl, "[ALERT_TRIGGERED_OR_RESOLVED]", "RESOLVED")
		} else {
			providerUrl = strings.ReplaceAll(providerUrl, "[ALERT_TRIGGERED_OR_RESOLVED]", "TRIGGERED")
		}
	}
	bodyBuffer := bytes.NewBuffer([]byte(body))
	request, _ := http.NewRequest(provider.Method, providerUrl, bodyBuffer)
	for k, v := range provider.Headers {
		request.Header.Set(k, v)
	}
	return request
}

func (provider *CustomAlertProvider) Send(serviceName, alertDescription string, resolved bool) error {
	request := provider.buildRequest(serviceName, alertDescription, resolved)
	response, err := client.GetHttpClient().Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode > 399 {
		return fmt.Errorf("call to provider alert returned status code %d", response.StatusCode)
	}
	return nil
}

func CreateSlackCustomAlertProvider(slackWebHookUrl string, service *Service, alert *Alert, result *Result, resolved bool) *CustomAlertProvider {
	var message string
	var color string
	if resolved {
		message = fmt.Sprintf("An alert for *%s* has been resolved after %d failures in a row", service.Name, service.NumberOfFailuresInARow)
		color = "#36A64F"
	} else {
		message = fmt.Sprintf("An alert for *%s* has been triggered", service.Name)
		color = "#DD0000"
	}
	var results string
	for _, conditionResult := range result.ConditionResults {
		var prefix string
		if conditionResult.Success {
			prefix = ":heavy_check_mark:"
		} else {
			prefix = ":x:"
		}
		results += fmt.Sprintf("%s - `%s`\n", prefix, conditionResult.Condition)
	}
	return &CustomAlertProvider{
		Url:    slackWebHookUrl,
		Method: "POST",
		Body: fmt.Sprintf(`{
  "text": "",
  "attachments": [
    {
      "title": ":helmet_with_white_cross: Gatus",
      "text": "%s:\n> %s",
      "short": false,
      "color": "%s",
      "fields": [
        {
          "title": "Condition results",
          "value": "%s",
          "short": false
        }
      ]
    },
  ]
}`, message, alert.Description, color, results),
		Headers: map[string]string{"Content-Type": "application/json"},
	}
}

func CreateTwilioCustomAlertProvider(provider *TwilioAlertProvider, message string) *CustomAlertProvider {
	return &CustomAlertProvider{
		Url:    fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", provider.SID),
		Method: "POST",
		Body: url.Values{
			"To":   {provider.To},
			"From": {provider.From},
			"Body": {message},
		}.Encode(),
		Headers: map[string]string{
			"Content-Type":  "application/x-www-form-urlencoded",
			"Authorization": fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", provider.SID, provider.Token)))),
		},
	}
}
