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

package devicerepo

import (
	"time"

	"github.com/SENERGY-Platform/device-repository/v3/lib/client"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
)

const AttributeUsedForZigbee = "senergy/zigbee-dc"

func (this *DeviceRepo) ListZigbeeDeviceTypes() (list []model.DeviceType, err error) {
	age := time.Since(this.lastDtRefresh)
	if age > this.maxCacheDuration {
		err = this.refreshDeviceTypeList()
		if err != nil {
			return nil, err
		}
	}
	return this.getDeviceTypeList(), nil
}

func (this *DeviceRepo) refreshDeviceTypeList() error {
	this.dtMux.Lock()
	defer this.dtMux.Unlock()
	result, err := this.getDeviceTypeListFromPlatform()
	if err != nil {
		this.config.GetLogger().Warn("unable to load device-types from platform", "error", err)
		return err
	}
	this.deviceTypes = result
	this.lastDtRefresh = time.Now()
	return nil
}

func (this *DeviceRepo) getDeviceTypeListFromPlatform() (result []model.DeviceType, err error) {
	token, err := this.getToken()
	if err != nil {
		return result, err
	}
	list, _, err, _ := this.repoclient.ListDeviceTypesV3(token, client.DeviceTypeListOptions{
		Limit:         9999,
		Offset:        0,
		AttributeKeys: []string{AttributeUsedForZigbee},
	})
	if err != nil {
		return result, err
	}
	for _, dt := range list {
		services := []model.Service{}
		for _, service := range dt.Services {
			services = append(services, model.Service{
				Id:          service.Id,
				LocalId:     service.LocalId,
				Name:        service.Name,
				Interaction: service.Interaction,
				Attributes:  service.Attributes,
			})
		}
		result = append(result, model.DeviceType{
			Id:          dt.Id,
			Name:        dt.Name,
			Description: dt.Description,
			Attributes:  dt.Attributes,
			Services:    services,
		})
	}
	return result, nil
}

func (this *DeviceRepo) getDeviceTypeList() []model.DeviceType {
	this.dtMux.Lock()
	defer this.dtMux.Unlock()
	return this.deviceTypes
}
