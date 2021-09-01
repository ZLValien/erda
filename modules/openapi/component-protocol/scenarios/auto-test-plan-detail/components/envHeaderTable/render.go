// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package envHeaderTable

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda/apistructs"
	protocol "github.com/erda-project/erda/modules/openapi/component-protocol"
)

type ComponentFileInfo struct {
	CtxBdl protocol.ContextBundle

	CommonFileInfo
}

type CommonFileInfo struct {
	Version    string                                           `json:"version,omitempty"`
	Name       string                                           `json:"name,omitempty"`
	Type       string                                           `json:"type,omitempty"`
	Props      map[string]interface{}                           `json:"props,omitempty"`
	State      State                                            `json:"state,omitempty"`
	Operations map[apistructs.OperationKey]apistructs.Operation `json:"operations,omitempty"`
	Data       map[string]interface{}                           `json:"data,omitempty"`
}

type DataList struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type PropColumn struct {
	Title     string `json:"title"`
	DataIndex string `json:"dataIndex"`
}

type State struct {
	EnvData apistructs.AutoTestGlobalConfig
}

func (a *ComponentFileInfo) Import(c *apistructs.Component) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}
	return nil
}

func (i *ComponentFileInfo) RenderProtocol(c *apistructs.Component, g *apistructs.GlobalStateData) {
	if c.Data == nil {
		d := make(apistructs.ComponentData)
		c.Data = d
	}
	(*c).Data["data"] = i.Data
	c.Props = i.Props

}

func (i *ComponentFileInfo) Render(ctx context.Context, c *apistructs.Component, _ apistructs.ComponentProtocolScenario, event apistructs.ComponentEvent, gs *apistructs.GlobalStateData) (err error) {
	if err := i.Import(c); err != nil {
		logrus.Errorf("import component failed, err:%v", err)
		return err
	}

	i.CtxBdl = ctx.Value(protocol.GlobalInnerKeyCtxBundle.String()).(protocol.ContextBundle)

	defer func() {
		fail := i.marshal(c)
		if err == nil && fail != nil {
			err = fail
		}
	}()
	i.Props = make(map[string]interface{})
	i.Props["fields"] = []PropColumn{
		{
			Title:     "名称",
			DataIndex: "name",
		},
		{
			Title:     "参数内容",
			DataIndex: "content",
		},
	}
	i.Data = make(map[string]interface{})
	var list []DataList
	for k, v := range i.State.EnvData.APIConfig.Header {
		list = append(list, DataList{
			Name:    k,
			Content: v,
		})
	}
	i.Data["list"] = list

	i.RenderProtocol(c, gs)
	return
}

func (a *ComponentFileInfo) marshal(c *apistructs.Component) error {
	stateValue, err := json.Marshal(a.State)
	if err != nil {
		return err
	}
	var state map[string]interface{}
	err = json.Unmarshal(stateValue, &state)
	if err != nil {
		return err
	}

	propValue, err := json.Marshal(a.Props)
	if err != nil {
		return err
	}
	var props interface{}
	err = json.Unmarshal(propValue, &props)
	if err != nil {
		return err
	}

	c.Props = props
	c.State = state
	c.Type = a.Type
	return nil
}

func RenderCreator() protocol.CompRender {
	return &ComponentFileInfo{
		CtxBdl: protocol.ContextBundle{},
		CommonFileInfo: CommonFileInfo{
			Props:      map[string]interface{}{},
			Operations: map[apistructs.OperationKey]apistructs.Operation{},
			Data:       map[string]interface{}{},
		},
	}
}
