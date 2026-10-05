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
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/SENERGY-Platform/device-repository/v3/lib/client"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/model"
	"github.com/SENERGY-Platform/models/go/models"
)

type DeviceRepo struct {
	config                  configuration.Config
	auth                    Auth
	deviceTypes             []model.DeviceType
	minCacheDuration        time.Duration
	maxCacheDuration        time.Duration
	lastDtRefresh           time.Time
	dtMux                   sync.Mutex
	createdDt               map[string]models.DeviceType
	repoclient              client.Interface
	deviceDeviceTypeIdCache map[string]string
}

type Auth interface {
	EnsureAccess(config configuration.Config) (token string, err error)
}

func New(config configuration.Config, auth Auth) (*DeviceRepo, error) {
	repoclient := client.NewClient(config.DeviceRepositoryUrl, func() (token string, err error) {
		return auth.EnsureAccess(config)
	})
	return NewWithDependencies(config, auth, repoclient)
}

func NewWithDependencies(config configuration.Config, auth Auth, repoclient client.Interface) (*DeviceRepo, error) {
	minCacheDuration, err := time.ParseDuration(config.MinCacheDuration)
	if err != nil {
		return nil, err
	}
	maxCacheDuration, err := time.ParseDuration(config.MaxCacheDuration)
	if err != nil {
		return nil, err
	}
	return &DeviceRepo{
		auth:                    auth,
		config:                  config,
		repoclient:              repoclient,
		minCacheDuration:        minCacheDuration,
		maxCacheDuration:        maxCacheDuration,
		createdDt:               map[string]models.DeviceType{},
		deviceDeviceTypeIdCache: map[string]string{},
	}, nil
}

func (this *DeviceRepo) getToken() (string, error) {
	if !this.config.AuthEnabled() {
		return "", nil
	}
	return this.auth.EnsureAccess(this.config)
}

func (this *DeviceRepo) getDeviceFromRepo(deviceId string) (device models.Device, known bool, err error) {
	token, err := this.getToken()
	if err != nil {
		return device, false, err
	}
	list, err, _ := this.repoclient.ListDevices(token, client.DeviceListOptions{LocalIds: []string{deviceId}})
	if err != nil {
		return device, false, err
	}
	if len(list) == 0 {
		return device, false, nil
	}
	device = list[0]
	return device, true, nil
}

func (this *DeviceRepo) GetKnownDeviceDeviceTypeId(deviceId string) (dtId string, known bool, err error) {
	dtId, known = this.deviceDeviceTypeIdCache[deviceId]
	if known {
		return dtId, true, nil
	}
	device, known, err := this.getDeviceFromRepo(deviceId)
	if err != nil {
		return "", false, err
	}
	if !known {
		return "", false, nil
	}
	dtId = device.DeviceTypeId
	this.deviceDeviceTypeIdCache[deviceId] = dtId
	return dtId, true, nil
}

func (this *DeviceRepo) FindDeviceTypeId(device model.ZigbeeDeviceInfo) (dtId string, err error) {
	deviceTypes, err := this.ListZigbeeDeviceTypes()
	if err != nil {
		return "", err
	}
	deviceType, ok := getMatchingDeviceType(deviceTypes, device)
	if !ok && time.Since(this.lastDtRefresh) > this.minCacheDuration {
		err = this.refreshDeviceTypeList()
		if err != nil {
			return "", err
		}
		deviceTypes, err = this.ListZigbeeDeviceTypes()
		if err != nil {
			return "", err
		}
		deviceType, ok = getMatchingDeviceType(deviceTypes, device)
	}
	if !ok {
		return "", fmt.Errorf("%w: vendor=%v model=%v", model.NoMatchingDeviceTypeFound, device.Definition.Vendor, device.Definition.Model)
	}
	return deviceType.Id, nil
}

func (this *DeviceRepo) FindDeviceType(device model.ZigbeeDeviceInfo) (dt model.DeviceType, err error) {
	deviceTypes, err := this.ListZigbeeDeviceTypes()
	if err != nil {
		return dt, err
	}
	deviceType, ok := getMatchingDeviceType(deviceTypes, device)
	if !ok && time.Since(this.lastDtRefresh) > this.minCacheDuration {
		err = this.refreshDeviceTypeList()
		if err != nil {
			return dt, err
		}
		deviceTypes, err = this.ListZigbeeDeviceTypes()
		if err != nil {
			return dt, err
		}
		deviceType, ok = getMatchingDeviceType(deviceTypes, device)
	}
	if !ok {
		return dt, fmt.Errorf("%w: vendor=%v model=%v", model.NoMatchingDeviceTypeFound, device.Definition.Vendor, device.Definition.Model)
	}
	return deviceType, nil
}

const AttributeZigbeeVendor = "senergy/zigbee-vendor"
const AttributeZigbeeModel = "senergy/zigbee-model"

func getMatchingDeviceType(devicetypes []model.DeviceType, device model.ZigbeeDeviceInfo) (model.DeviceType, bool) {
	for _, dt := range devicetypes {
		attrMap := map[string][]string{}
		for _, attr := range dt.Attributes {
			attrMap[attr.Key] = append(attrMap[attr.Key], strings.ToLower(strings.TrimSpace(attr.Value)))
		}
		if vendors, vendorIsSet := attrMap[AttributeZigbeeVendor]; vendorIsSet && slices.Contains(vendors, strings.ToLower(strings.TrimSpace(device.Definition.Vendor))) {
			if modelList, modelIsSet := attrMap[AttributeZigbeeModel]; modelIsSet && slices.Contains(modelList, strings.ToLower(strings.TrimSpace(device.Definition.Model))) {
				return dt, true
			}
		}
	}
	return model.DeviceType{}, false
}
