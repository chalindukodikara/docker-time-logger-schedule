/*
 * Copyright (c) 2023, WSO2 LLC. (https://www.wso2.com/) All Rights Reserved.
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package main

import (
	"log"
	"time"
)

func main() {
	// Get the current time
	currentTime := time.Now()

	// Format the current time as a string
	timeString := currentTime.Format("2006-01-02 15:04:05")

	// Log the current time
	log.Printf("Current time: %s", timeString)

	x := BuildConfiguration{
		Docker: Docker{
			Context:        "context",
			DockerfilePath: "dockerfile",
		},
	}

	// print x in json format
	log.Printf("BuildConfiguration: %v", x)
	log.Printf("After adding the commit: 12.41 PM")
}

type BuildConfiguration struct {
	Docker    Docker    `json:"docker,omitempty"`
	Buildpack Buildpack `json:"buildpack,omitempty"`
}

type Docker struct {
	Context        string `json:"context,omitempty"`
	DockerfilePath string `json:"dockerfilePath,omitempty"`
}

type Buildpack struct {
	Name    string `json:"name"`
	Version string        `json:"version,omitempty"`
}
