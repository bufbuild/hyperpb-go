// Copyright 2025 Buf Technologies, Inc.
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

package hyperpb_test

import (
	"fmt"

	weatherv1 "buf.build/gen/go/bufbuild/hyperpb-examples/protocolbuffers/go/example/weather/v1"
	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"buf.build/go/hyperpb"
	"buf.build/go/hyperpb/internal/examples"
)

// Example data matches the data in the README example and should be kept in sync.
func Example() {
	// Compile a type for your message. This operation is quite slow, so it
	// should be cached, like regexp.Compile.
	ty := hyperpb.CompileMessageDescriptor((*weatherv1.WeatherReport)(nil).ProtoReflect().Descriptor())

	data := examples.ReadWeatherData() // Read some raw Protobuf-encoded data.

	// Unmarshal it, just how you normally would.
	msg := hyperpb.NewMessage(ty)
	if err := proto.Unmarshal(data, msg); err != nil {
		panic(err)
	}

	// Use reflection to read some fields.
	fields := ty.Descriptor().Fields()
	fmt.Println(msg.Get(fields.ByName("region")))
	stations := msg.Get(fields.ByName("weather_stations")).List()
	for i := range stations.Len() {
		station := stations.Get(i).Message()
		fields := station.Descriptor().Fields()

		fmt.Println("station:", station.Get(fields.ByName("station")))
		fmt.Println("frequency:", station.Get(fields.ByName("frequency")))
		fmt.Println("temperature:", station.Get(fields.ByName("temperature")))
		fmt.Println("pressure:", station.Get(fields.ByName("pressure")))
		fmt.Println("wind_speed:", station.Get(fields.ByName("wind_speed")))
		fmt.Println("conditions:", station.Get(fields.ByName("conditions")))
	}

	// Output:
	// Seattle
	// station: KAD93
	// frequency: 162.525
	// temperature: 11.3
	// pressure: 30.08
	// wind_speed: 2.3
	// conditions: 3
	// station: KHB60
	// frequency: 162.55
	// temperature: 13.7
	// pressure: 28.09
	// wind_speed: 1.9
	// conditions: 3
}

func Example_protovalidate() {
	// Compile a type for your message. This operation is quite slow, so it
	// should be cached, like regexp.Compile.
	ty := hyperpb.CompileMessageDescriptor((*weatherv1.WeatherReport)(nil).ProtoReflect().Descriptor())

	data := examples.ReadWeatherData() // Read some raw Protobuf-encoded data.

	// Unmarshal it, just how you normally would.
	msg := hyperpb.NewMessage(ty)
	if err := proto.Unmarshal(data, msg); err != nil {
		panic(err)
	}

	// It just works!
	err := protovalidate.Validate(msg)

	fmt.Println("error:", err)

	// Output:
	// error: <nil>
}

func Example_unmarshalFromDescriptor() {
	// Download a descriptor off of the network, unmarshal it, and compile a
	// type from it.
	fds := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(examples.DownloadWeatherReportSchema(), fds); err != nil {
		panic(err)
	}

	ty, err := hyperpb.CompileFileDescriptorSet(fds,
		"example.weather.v1.WeatherReport", // The type we want to compile.
	)
	if err != nil {
		panic(err)
	}

	data := examples.ReadWeatherData() // Read some raw Protobuf-encoded data.

	// Unmarshal it, just how you normally would.
	msg := hyperpb.NewMessage(ty)
	if err := proto.Unmarshal(data, msg); err != nil {
		panic(err)
	}

	// Use reflection to read some fields.
	fields := ty.Descriptor().Fields()
	fmt.Println(msg.Get(fields.ByName("region")))
	stations := msg.Get(fields.ByName("weather_stations")).List()
	for i := range stations.Len() {
		station := stations.Get(i).Message()
		fields := station.Descriptor().Fields()

		fmt.Println("station:", station.Get(fields.ByName("station")))
		fmt.Println("frequency:", station.Get(fields.ByName("frequency")))
		fmt.Println("temperature:", station.Get(fields.ByName("temperature")))
		fmt.Println("pressure:", station.Get(fields.ByName("pressure")))
		fmt.Println("wind_speed:", station.Get(fields.ByName("wind_speed")))
		fmt.Println("conditions:", station.Get(fields.ByName("conditions")))
	}

	// Output:
	// Seattle
	// station: KAD93
	// frequency: 162.525
	// temperature: 11.3
	// pressure: 30.08
	// wind_speed: 2.3
	// conditions: 3
	// station: KHB60
	// frequency: 162.55
	// temperature: 13.7
	// pressure: 28.09
	// wind_speed: 1.9
	// conditions: 3
}
