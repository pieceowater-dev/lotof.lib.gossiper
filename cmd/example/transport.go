package main

import (
	gossiper "github.com/pieceowater-dev/lotof.lib.gossiper"
)

type SomeService struct {
	transport gossiper.Transport
	//client    pb.SomeServiceClient // Add client as a property, generated from protobuf
}

func NewSomeService() *SomeService {
	factory := gossiper.NewTransportFactory()
	grpcTransport := factory.CreateTransport(
		gossiper.GRPC,
		"localhost:50051",
	)

	// Create the client only once and store it as a property
	//clientConstructor := pb.NewSomeServiceClient
	//client, err := grpcTransport.CreateClient(clientConstructor)
	//if err != nil {
	//	log.Fatalf("Error creating client: %v", err)
	//}

	return &SomeService{
		transport: grpcTransport,
		//client:    client,
	}
}

//func (s *SomeService) Items() ([]any, error) {
//	ctx := context.Background()
//
//	// Send the request using the client stored in the SomeService instance
//	response, err := s.transport.Send(
//		ctx,
//		s.client,
//		"GetItems",
//		&pb.GetItemsRequest{}, // Dynamic request for GetItems
//	)
//	if err != nil {
//		log.Printf("Error sending request: %v", err)
//		return nil, err
//	}
//
//	// Assert the response to the correct type
//	res, ok := response.(*pb.GetItemsResponse)
//	if !ok {
//		return nil, errors.New("invalid response type from gRPC transport")
//	}
//
//	return res, nil
//}
