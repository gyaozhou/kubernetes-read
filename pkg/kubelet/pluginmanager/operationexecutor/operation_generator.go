/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package operationexecutor implements interfaces that enable execution of
// register and unregister operations with a
// goroutinemap so that more than one operation is never triggered
// on the same plugin.
package operationexecutor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/client-go/tools/record"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
	"k8s.io/kubernetes/pkg/kubelet/pluginmanager/cache"
)

const (
	dialTimeoutDuration   = 10 * time.Second
	notifyTimeoutDuration = 5 * time.Second
)

var _ OperationGenerator = &operationGenerator{}

type operationGenerator struct {

	// recorder is used to record events in the API server
	recorder record.EventRecorder
}

// NewOperationGenerator is returns instance of operationGenerator
func NewOperationGenerator(recorder record.EventRecorder) OperationGenerator {

	return &operationGenerator{
		recorder: recorder,
	}
}

// zhou: in order to test OperationExecutor core logic, "OperationGenerator" is used to work as interface to
//       decouple dependecy. "fakeOGCounter{}" is used in this case.
//       Just generate callback function used by OperationExecutor.

// OperationGenerator interface that extracts out the functions from operation_executor to make it dependency injectable
type OperationGenerator interface {
	// Generates the RegisterPlugin function needed to perform the registration of a plugin
	GenerateRegisterPluginFunc(
		socketPath string,
		UUID types.UID,
		pluginHandlers map[string]cache.PluginHandler,
		actualStateOfWorldUpdater ActualStateOfWorldUpdater) func() error

	// Generates the UnregisterPlugin function needed to perform the unregistration of a plugin
	GenerateUnregisterPluginFunc(
		pluginInfo cache.PluginInfo,
		actualStateOfWorldUpdater ActualStateOfWorldUpdater) func() error
}

// zhou: generate a callback function for register Plugin

func (og *operationGenerator) GenerateRegisterPluginFunc(
	socketPath string,
	pluginUUID types.UID,
	pluginHandlers map[string]cache.PluginHandler,
	actualStateOfWorldUpdater ActualStateOfWorldUpdater) func() error {

	registerPluginFunc := func() error {

		// zhou: connect to registration unix sock file, like
		//       "/var/lib/kubelet/plugins_registry/<driverName>-reg.sock"

		client, conn, err := dial(socketPath, dialTimeoutDuration)
		if err != nil {
			return fmt.Errorf("RegisterPlugin error -- dial failed at socket %s, err: %v", socketPath, err)
		}
		defer conn.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// zhou: e.g. RPC invoke csi-node-driver-registrar/GetInfo(),
		//       including Plugin type, name and endpoint.

		infoResp, err := client.GetInfo(ctx, &registerapi.InfoRequest{})
		if err != nil {
			return fmt.Errorf("RegisterPlugin error -- failed to get plugin info using RPC GetInfo at socket %s, err: %v", socketPath, err)
		}

		// zhou: find the plugin handler for each Plugin type

		handler, ok := pluginHandlers[infoResp.Type]
		if !ok {
			if err := og.notifyPlugin(client, false, fmt.Sprintf("RegisterPlugin error -- no handler registered for plugin type: %s at socket %s", infoResp.Type, socketPath)); err != nil {
				return fmt.Errorf("RegisterPlugin error -- failed to send error at socket %s, err: %v", socketPath, err)
			}
			return fmt.Errorf("RegisterPlugin error -- no handler registered for plugin type: %s at socket %s", infoResp.Type, socketPath)
		}

		// zhou: if not provided, reuse the registration sock file.

		if infoResp.Endpoint == "" {
			infoResp.Endpoint = socketPath
		}

		// zhou: e.g. call csi_plugin.go/ValidatePlugin()

		if err := handler.ValidatePlugin(infoResp.Name, infoResp.Endpoint, infoResp.SupportedVersions); err != nil {
			if err = og.notifyPlugin(client, false, fmt.Sprintf("RegisterPlugin error -- plugin validation failed with err: %v", err)); err != nil {
				return fmt.Errorf("RegisterPlugin error -- failed to send error at socket %s, err: %v", socketPath, err)
			}
			return fmt.Errorf("RegisterPlugin error -- pluginHandler.ValidatePluginFunc failed")
		}

		// zhou: add to ASW

		// We add the plugin to the actual state of world cache before calling a plugin consumer's Register handle
		// so that if we receive a delete event during Register Plugin, we can process it as a DeRegister call.
		err = actualStateOfWorldUpdater.AddPlugin(cache.PluginInfo{
			SocketPath: socketPath,
			UUID:       pluginUUID,
			Handler:    handler,
			Name:       infoResp.Name,
			Endpoint:   infoResp.Endpoint,
		})
		if err != nil {
			klog.ErrorS(err, "RegisterPlugin error -- failed to add plugin", "path", socketPath)
		}

		// zhou: e.g. call csi_plugin.go/RegisterPlugin()

		if err := handler.RegisterPlugin(infoResp.Name, infoResp.Endpoint, infoResp.SupportedVersions, nil); err != nil {
			return og.notifyPlugin(client, false, fmt.Sprintf("RegisterPlugin error -- plugin registration failed with err: %v", err))
		}

		// zhou: e.g. RPC invoke csi-node-driver-registrar/NotifyRegistrationStatus(),

		// Notify is called after register to guarantee that even if notify throws an error Register will always be called after validate
		if err := og.notifyPlugin(client, true, ""); err != nil {
			return fmt.Errorf("RegisterPlugin error -- failed to send registration status at socket %s, err: %v", socketPath, err)
		}
		return nil
	}
	return registerPluginFunc
}

// zhou: generate a callback function for unregister Plugin

func (og *operationGenerator) GenerateUnregisterPluginFunc(
	pluginInfo cache.PluginInfo,
	actualStateOfWorldUpdater ActualStateOfWorldUpdater) func() error {

	unregisterPluginFunc := func() error {
		if pluginInfo.Handler == nil {
			return fmt.Errorf("UnregisterPlugin error -- failed to get plugin handler for %s", pluginInfo.SocketPath)
		}
		// We remove the plugin to the actual state of world cache before calling a plugin consumer's Unregister handle
		// so that if we receive a register event during Register Plugin, we can process it as a Register call.
		actualStateOfWorldUpdater.RemovePlugin(pluginInfo.SocketPath)

		pluginInfo.Handler.DeRegisterPlugin(pluginInfo.Name, pluginInfo.Endpoint)

		klog.V(4).InfoS("DeRegisterPlugin called", "pluginName", pluginInfo.Name, "pluginHandler", pluginInfo.Handler)
		return nil
	}
	return unregisterPluginFunc
}

// zhou:

func (og *operationGenerator) notifyPlugin(client registerapi.RegistrationClient, registered bool, errStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeoutDuration)
	defer cancel()

	status := &registerapi.RegistrationStatus{
		PluginRegistered: registered,
		Error:            errStr,
	}

	// zhou: e.g. RPC invoke csi-node-driver-registrar/NotifyRegistrationStatus(),

	if _, err := client.NotifyRegistrationStatus(ctx, status); err != nil {
		return fmt.Errorf("%s: %w", errStr, err)
	}

	if errStr != "" {
		return errors.New(errStr)
	}

	return nil
}

// Dial establishes the gRPC communication with the picked up plugin socket. https://godoc.org/google.golang.org/grpc#Dial
func dial(unixSocketPath string, timeout time.Duration) (registerapi.RegistrationClient, *grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c, err := grpc.DialContext(ctx, unixSocketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial socket %s, err: %v", unixSocketPath, err)
	}

	return registerapi.NewRegistrationClient(c), c, nil
}
