/*
 * Copyright (c) 2023 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/casbin/govaluate"
	"log"
	"sync"
)

func (this *Connector) startEventHandling(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("start event handling")
		for {
			select {
			case <-ctx.Done():
				log.Println("stop event handling")
				return
			case event := <-this.eventbuffer:
				this.handleEvent(event)
			}
		}
	}()
	return nil
}

func (this *Connector) handleEvent(event EventDesc) {
	if this.eventIsAllowed(event) {
		deviceid := this.getDeviceId(event.Device)
		serviceIds, err := this.getServiceIds(event)
		if err != nil {
			log.Println("ERROR: unable to get event service id", err)
			this.mgw.SendDeviceError(deviceid, "unable to get event service id: "+err.Error())
		}
		for _, serviceId := range serviceIds {
			err = this.mgw.SendEvent(deviceid, serviceId, event.Payload)
			if err != nil {
				log.Println("ERROR: unable to send event to mgw", err)
				this.mgw.SendDeviceError(deviceid, "unable to send event to mgw: "+err.Error())
			}
		}
	}
}

func (this *Connector) eventIsAllowed(event EventDesc) bool {
	return !event.Device.Disabled && event.Device.Supported && event.Device.InterviewCompleted && this.deviceIsOnline(event.Device.IeeeAddress)
}

func (this *Connector) getServiceIds(event EventDesc) ([]string, error) {
	dt, _, err := this.devicerepo.FindDeviceType(event.Device)
	if err != nil {
		return nil, err
	}
	return getServiceIds(dt, event.Payload)
}

func getServiceIds(dt model.DeviceType, pl []byte) ([]string, error) {
	result := []string{"get"}
	var value interface{}
	err := json.Unmarshal(pl, &value)
	if err != nil {
		return nil, err
	}
	for _, service := range dt.Services {
		ok, err := checkServiceAttrCondition(service, value)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, service.LocalId)
		}
	}
	return result, nil
}

const MgwZigbeeServiceSelectionConditionAttrKey = "mgw-service-selection-condition"

func checkServiceAttrCondition(service model.Service, value interface{}) (ok bool, err error) {
	for _, attr := range service.Attributes {
		if attr.Key == MgwZigbeeServiceSelectionConditionAttrKey {
			ok, err = evalServiceSelectionCondition(attr.Value, value)
			if err != nil {
				log.Printf("WARNING: unable to check service condition script=%#v value=%#v err=%#v", attr.Value, value, err.Error())
				return false, nil
			}
			return ok, nil
		}
	}
	return false, nil
}

func evalServiceSelectionCondition(condition string, value interface{}) (bool, error) {
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(condition, map[string]govaluate.ExpressionFunction{
		"contains": func(args ...interface{}) (interface{}, error) {
			if len(args) != 2 {
				return nil, errors.New("contains: expect exactly 2 arguments")
			}
			obj := args[0]
			key, ok := args[1].(string)
			if !ok {
				return nil, errors.New("contains: expect argument 2 to be a string")
			}

			switch v := obj.(type) {
			case map[string]interface{}:
				_, ok := v[key]
				return ok, nil
			case []interface{}:
				for _, item := range v {
					itemStr, ok := item.(string)
					if ok && itemStr == key {
						return true, nil
					}
				}
				return false, nil
			default:
				return false, nil
			}
		},
	})
	if err != nil {
		return false, err
	}
	resultInterface, err := expression.Evaluate(map[string]interface{}{
		"value": value,
	})
	if err != nil {
		return false, err
	}
	result, ok := resultInterface.(bool)
	if !ok {
		return false, fmt.Errorf("expected boolean result on evaluating service selection condition %v, got %T", condition, resultInterface)
	}
	return result, nil
}
