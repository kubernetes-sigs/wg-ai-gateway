/*
Copyright 2025 The Kubernetes Authors.

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

// guardrail-processor is a sample Envoy ext_proc server that demonstrates
// payload processing for AI gateway use cases.
//
// It inspects request bodies for blocked patterns (e.g., SQL injection
// attempts) and rejects matching requests with a 403 Forbidden response.
// All other requests are passed through unmodified.
//
// This is a reference implementation — replace the pattern matching with
// your own guardrail logic (LLM-based content moderation, PII detection, etc.).
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ext_procv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc"
)

const listenAddr = ":50051"

// blockedPatterns are substrings that cause the request to be rejected.
// In a real implementation, replace this with an LLM call, regex engine, or external service.
var blockedPatterns = []string{
	"DROP TABLE",
	"DELETE FROM",
	"<script>",
	"ignore previous instructions",
}

type server struct {
	ext_procv3.UnimplementedExternalProcessorServer
}

func (s *server) Process(stream ext_procv3.ExternalProcessor_ProcessServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := &ext_procv3.ProcessingResponse{}

		switch v := req.Request.(type) {
		case *ext_procv3.ProcessingRequest_RequestHeaders:
			// Pass request headers through without modification
			resp.Response = &ext_procv3.ProcessingResponse_RequestHeaders{
				RequestHeaders: &ext_procv3.HeadersResponse{},
			}

		case *ext_procv3.ProcessingRequest_RequestBody:
			body := string(v.RequestBody.Body)

			// Check for blocked patterns
			if reason := checkBlocked(body); reason != "" {
				// Reject the request with an immediate 403 response
				resp.Response = &ext_procv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &ext_procv3.ImmediateResponse{
						Status: &typev3.HttpStatus{Code: typev3.StatusCode_Forbidden},
						Headers: &ext_procv3.HeaderMutation{
							SetHeaders: []*corev3.HeaderValueOption{
								{
									Header: &corev3.HeaderValue{
										Key:   "content-type",
										Value: "application/json",
									},
								},
							},
						},
						Body: []byte(fmt.Sprintf(`{"error": "request blocked", "reason": %q}`, reason)),
					},
				}
			} else {
				// Allow the request through unmodified
				resp.Response = &ext_procv3.ProcessingResponse_RequestBody{
					RequestBody: &ext_procv3.BodyResponse{},
				}
			}

		case *ext_procv3.ProcessingRequest_ResponseHeaders:
			resp.Response = &ext_procv3.ProcessingResponse_ResponseHeaders{
				ResponseHeaders: &ext_procv3.HeadersResponse{},
			}

		case *ext_procv3.ProcessingRequest_ResponseBody:
			resp.Response = &ext_procv3.ProcessingResponse_ResponseBody{
				ResponseBody: &ext_procv3.BodyResponse{},
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func checkBlocked(body string) string {
	upper := strings.ToUpper(body)
	for _, pattern := range blockedPatterns {
		if strings.Contains(upper, strings.ToUpper(pattern)) {
			return fmt.Sprintf("blocked pattern detected: %s", pattern)
		}
	}
	return ""
}

func main() {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	ext_procv3.RegisterExternalProcessorServer(grpcServer, &server{})

	log.Printf("guardrail-processor listening on %s", listenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
