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

package model

import "errors"

var ErrNotFound = errors.New("not found")
var ErrNoValueFound = errors.New("no value found")
var ErrNotReady = errors.New("not ready")
var NoMatchingDeviceTypeFound = errors.New("unable to find matching device type")

type ZigbeeDeviceInfo struct {
	IeeeAddress        string                     `json:"ieee_address"`
	FriendlyName       string                     `json:"friendly_name"`
	Manufacturer       string                     `json:"manufacturer"` //do not use for senergy-device-type identification. instead use ZigbeeDeviceInfoDefinition.Vendor
	ModelId            string                     `json:"model_id"`     //do not use for senergy-device-type identification. instead use ZigbeeDeviceInfoDefinition.Model
	Supported          bool                       `json:"supported"`
	Disabled           bool                       `json:"disabled"`
	InterviewCompleted bool                       `json:"interview_completed"`
	Definition         ZigbeeDeviceInfoDefinition `json:"definition"`
}

type ZigbeeDeviceInfoDefinition struct {
	Description string                    `json:"description"`
	Model       string                    `json:"model"`
	Vendor      string                    `json:"vendor"`
	Exposes     []ZigbeeDeviceInfoFeature `json:"exposes"`
}

type ZigbeeDeviceInfoFeature struct {
	Type        string                          `json:"type"`
	Name        string                          `json:"name,omitempty"`
	Property    string                          `json:"property,omitempty"`
	Access      int                             `json:"access,omitempty"`
	Description string                          `json:"description,omitempty"`
	Features    []ZigbeeDeviceInfoFeature       `json:"features,omitempty"`
	ItemType    []ZigbeeDeviceInfoFeature       `json:"item_type,omitempty"`
	Unit        string                          `json:"unit,omitempty"`
	Values      []interface{}                   `json:"values,omitempty"`
	ValueMax    int64                           `json:"value_max,omitempty"`
	ValueMin    int64                           `json:"value_min,omitempty"`
	ValueOff    interface{}                     `json:"value_off,omitempty"`
	ValueOn     interface{}                     `json:"value_on,omitempty"`
	ValueToggle interface{}                     `json:"value_toggle,omitempty"`
	Presets     []ZigbeeDeviceInfoFeaturePreset `json:"presets,omitempty"`
}

type ZigbeeDeviceInfoFeaturePreset struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Value       interface{} `json:"value"`
}

type DeviceType struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Attributes  []Attribute `json:"attributes"`
}

type Attribute struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Origin string `json:"origin"`
}
