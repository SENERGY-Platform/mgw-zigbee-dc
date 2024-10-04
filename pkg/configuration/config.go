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

package configuration

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Debug bool `json:"debug"`

	ConnectorId string `json:"connector_id"`

	DeviceIdPrefix string `json:"device_id_prefix"`

	MgwMqttBroker   string `json:"mgw_mqtt_broker"`
	MgwMqttClientId string `json:"mgw_mqtt_client_id"`
	MgwMqttUser     string `json:"mgw_mqtt_user" config:"secret"`
	MgwMqttPw       string `json:"mgw_mqtt_pw" config:"secret"`

	ZigbeeMqttBroker   string `json:"zigbee_mqtt_broker"`
	ZigbeeMqttClientId string `json:"zigbee_mqtt_client_id"`
	ZigbeeMqttUser     string `json:"zigbee_mqtt_user" config:"secret"`
	ZigbeeMqttPw       string `json:"zigbee_mqtt_pw" config:"secret"`

	ZigbeeQos int `json:"zigbee_qos"`

	ZigbeeMqttTopicPrefix string `json:"zigbee_mqtt_topic_prefix"`

	EventRefreshInterval string `json:"event_refresh_interval"`

	AuthEndpoint             string  `json:"auth_endpoint"`
	AuthClientId             string  `json:"auth_client_id" config:"secret"`
	AuthExpirationTimeBuffer float64 `json:"auth_expiration_time_buffer"`
	AuthUsername             string  `json:"auth_username" config:"secret"`
	AuthPassword             string  `json:"auth_password" config:"secret"`

	DeviceManagerUrl    string `json:"device_manager_url"`
	DeviceRepositoryUrl string `json:"device_repository_url"`
	FallbackFile        string `json:"fallback_file"`
	MinCacheDuration    string `json:"min_cache_duration"`
	MaxCacheDuration    string `json:"max_cache_duration"`

	CreateMissingDeviceTypes                    bool   `json:"create_missing_device_types"`
	CreateMissingDeviceTypesWithDeviceClass     string `json:"create_missing_device_types_with_device_class"`
	CreateMissingDeviceTypesWithProtocol        string `json:"create_missing_device_types_with_protocol"`
	CreateMissingDeviceTypesWithProtocolSegment string `json:"create_missing_device_types_with_protocol_segment"`
}

// loads config from json in location and used environment variables (e.g ZookeeperUrl --> ZOOKEEPER_URL)
func Load(location string) (config Config, err error) {
	file, error := os.Open(location)
	if error != nil {
		log.Println("error on config load: ", error)
		return config, error
	}
	decoder := json.NewDecoder(file)
	error = decoder.Decode(&config)
	if error != nil {
		log.Println("invalid config json: ", error)
		return config, error
	}
	handleEnvironmentVars(&config)
	return config, nil
}

var camel = regexp.MustCompile("(^[^A-Z]*|[A-Z]*)([A-Z][^A-Z]+|$)")

func fieldNameToEnvName(s string) string {
	var a []string
	for _, sub := range camel.FindAllStringSubmatch(s, -1) {
		if sub[1] != "" {
			a = append(a, sub[1])
		}
		if sub[2] != "" {
			a = append(a, sub[2])
		}
	}
	return strings.ToUpper(strings.Join(a, "_"))
}

// preparations for docker
func handleEnvironmentVars(config *Config) {
	configValue := reflect.Indirect(reflect.ValueOf(config))
	configType := configValue.Type()
	for index := 0; index < configType.NumField(); index++ {
		fieldName := configType.Field(index).Name
		fieldConfig := configType.Field(index).Tag.Get("config")
		envName := fieldNameToEnvName(fieldName)
		envValue := os.Getenv(envName)
		if envValue != "" {
			loggedEnvValue := envValue
			if strings.Contains(fieldConfig, "secret") {
				loggedEnvValue = "***"
			}
			fmt.Println("use environment variable: ", envName, " = ", loggedEnvValue)
			if configValue.FieldByName(fieldName).Kind() == reflect.Int64 {
				i, _ := strconv.ParseInt(envValue, 10, 64)
				configValue.FieldByName(fieldName).SetInt(i)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.String {
				configValue.FieldByName(fieldName).SetString(envValue)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Bool {
				b, _ := strconv.ParseBool(envValue)
				configValue.FieldByName(fieldName).SetBool(b)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Float64 {
				f, _ := strconv.ParseFloat(envValue, 64)
				configValue.FieldByName(fieldName).SetFloat(f)
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Slice {
				val := []string{}
				for _, element := range strings.Split(envValue, ",") {
					val = append(val, strings.TrimSpace(element))
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(val))
			}
			if configValue.FieldByName(fieldName).Kind() == reflect.Map {
				value := map[string]string{}
				for _, element := range strings.Split(envValue, ",") {
					keyVal := strings.Split(element, ":")
					key := strings.TrimSpace(keyVal[0])
					val := strings.TrimSpace(keyVal[1])
					value[key] = val
				}
				configValue.FieldByName(fieldName).Set(reflect.ValueOf(value))
			}
		}
	}
}
