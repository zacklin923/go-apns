package server

import (
	"encoding/json"
	"errors"
	log "github.com/blackbeans/log4go"
	"go-apns/entry"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
)

var regx *regexp.Regexp

func init() {
	regx, _ = regexp.Compile("\\w+")
}

func (self *ApnsHttpServer) decodePayload(req *http.Request, resp *response) (string, *entry.PayLoad) {

	tokenV := req.PostFormValue("token")
	sound := req.PostFormValue("sound")
	badgeV := req.PostFormValue("badge")
	body := req.PostFormValue("body")

	//-----------检查参数

	noToken := checkArguments(tokenV)
	if !noToken {
		resp.Status = RESP_STATUS_PUSH_ARGUMENTS_INVALID
		resp.Error = errors.New("Notification Params are Invalid!|NO TOKEN!")
		return "", nil
	}

	valid := checkArgumentsNotNil(sound, badgeV, body)
	if !valid {
		resp.Status = RESP_STATUS_PUSH_ARGUMENTS_INVALID
		resp.Error = errors.New("Notification Params are Invalid!| PayLoad Lack!")
		return "", nil
	}

	tokenSplit := regx.FindAllString(tokenV, -1)
	var token string = ""
	for _, v := range tokenSplit {
		token += v
	}

	aps := entry.Aps{}
	if len(sound) > 0 {
		aps.Sound = sound
	}

	if len(badgeV) > 0 {
		badge, _ := strconv.ParseInt(badgeV, 10, 32)
		aps.Badge = int(badge)
	}

	if len(body) > 0 {
		aps.Alert = body
	}

	//拼接payload
	payload := entry.NewSimplePayLoadWithAps(aps)

	//是个大的Json数据即可
	extArgs := req.PostFormValue("extArgs")
	if len(extArgs) > 0 {
		var jsonMap map[string]interface{}
		err := json.Unmarshal([]byte(extArgs), &jsonMap)
		if nil != err {
			resp.Status = RESP_STATUS_PAYLOAD_BODY_DECODE_ERROR
			resp.Error = errors.New("PAYLOAD BODY DECODE ERROR!")
		} else {
			for k, v := range jsonMap {
				//如果存在数据嵌套则返回错误，不允许数据多层嵌套
				if reflect.TypeOf(v).Kind() == reflect.Map {
					resp.Status = RESP_STATUS_PAYLOAD_BODY_DEEP_ITERATOR
					resp.Error = errors.New("DEEP PAYLOAD BODY ITERATOR!")
					break
				} else {
					payload.AddExtParam(k, v)
				}
			}
		}
	}

	return token, payload
}

//内部发送代码
func (self *ApnsHttpServer) innerSend(pushType string, token string, payload *entry.PayLoad, resp *response, expiredTime uint32) {
	var sendFunc func() error

	if NOTIFY_SIMPLE_FORMAT == pushType {
		//如果为简单
		sendFunc = func() error {
			return self.apnsClient.SendSimpleNotification(token, *payload)
		}
	} else if NOTIFY_ENHANCED_FORMAT == pushType {
		//如果为扩展的
		sendFunc = func() error {
			return self.apnsClient.SendEnhancedNotification(expiredTime, token, *payload)
		}

	} else {
		resp.Status = RESP_STATUS_INVALID_NOTIFY_FORMAT
		resp.Error = errors.New("Invalid notification format " + pushType)
	}

	//能直接放在chan中异步发送
	var err error
	//如果有异常则重试发送
	if RESP_STATUS_SUCC == resp.Status {
		err = sendFunc()
		if nil == err {
			log.DebugLog("push_handler", "ApnsHttpServer|SendNotification|SUCC|FORMAT:%s|%s", pushType, *payload)
		}
	}
	if nil != err {
		log.ErrorLog("push_handler", "ApnsHttpServer|SendNotification|FORMAT:%s|FAIL|IGNORED|%s|%s", pushType, *payload, err)
		resp.Status = RESP_STATUS_SEND_OVER_TRY_ERROR
		resp.Error = err
	}
}

func checkArguments(args ...string) bool {
	for _, v := range args {
		if len(v) <= 0 {
			return false
		}
	}

	return true
}

func checkArgumentsNotNil(args ...string) bool {
	for _, v := range args {
		if len(v) > 0 {
			return true
		}
	}
	return false
}
