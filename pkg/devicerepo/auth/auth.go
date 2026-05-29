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

package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/SENERGY-Platform/mgw-zigbee-dc/pkg/configuration"
)

type Auth struct {
	AccessToken      string    `json:"access_token"`
	ExpiresIn        float64   `json:"expires_in"`
	RefreshExpiresIn float64   `json:"refresh_expires_in"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	RequestTime      time.Time `json:"-"`
}

func (openid *Auth) EnsureAccess(config configuration.Config) (token string, err error) {
	if !config.AuthEnabled() {
		return "", nil
	}
	duration := time.Now().Sub(openid.RequestTime).Seconds()

	if openid.AccessToken != "" && openid.ExpiresIn-config.AuthExpirationTimeBuffer > duration {
		token = "Bearer " + openid.AccessToken
		return
	}

	if openid.RefreshToken != "" && openid.RefreshExpiresIn-config.AuthExpirationTimeBuffer > duration {
		config.GetLogger().Debug("refresh token", "expires_in", openid.RefreshExpiresIn, "duration", duration)
		err = refreshOpenidToken(openid, config)
		if err != nil {
			config.GetLogger().Warn("unable to use refreshtoken", "error", err)
		} else {
			token = "Bearer " + openid.AccessToken
			return
		}
	}

	config.GetLogger().Debug("get new access token")
	err = getOpenidToken(openid, config)
	if err != nil {
		config.GetLogger().Error("unable to get new access token", "error", err)
		openid = &Auth{}
	}
	token = "Bearer " + openid.AccessToken
	return
}

func getOpenidToken(token *Auth, config configuration.Config) (err error) {
	requesttime := time.Now()
	var values url.Values
	values = url.Values{
		"client_id":  {config.AuthClientId},
		"username":   {config.AuthUsername},
		"password":   {config.AuthPassword},
		"grant_type": {"password"},
	}
	resp, err := http.PostForm(config.AuthEndpoint+"/auth/realms/master/protocol/openid-connect/token", values)

	if err != nil {
		config.GetLogger().Error("unable to get new access token", "error", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err = errors.New(string(body))
		resp.Body.Close()
		return
	}
	err = json.NewDecoder(resp.Body).Decode(token)
	token.RequestTime = requesttime
	return
}

func refreshOpenidToken(token *Auth, config configuration.Config) (err error) {
	requesttime := time.Now()
	resp, err := http.PostForm(config.AuthEndpoint+"/auth/realms/master/protocol/openid-connect/token", url.Values{
		"client_id":     {config.AuthClientId},
		"refresh_token": {token.RefreshToken},
		"grant_type":    {"refresh_token"},
	})

	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err = errors.New(string(body))
		resp.Body.Close()
		return
	}
	err = json.NewDecoder(resp.Body).Decode(token)
	token.RequestTime = requesttime
	return
}
