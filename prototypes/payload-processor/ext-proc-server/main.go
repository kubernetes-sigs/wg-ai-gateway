// POC: ExtProc server that reads the request body, extracts the "model" field,
// and sets it as the X-Gateway-Model-Name header on the request.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
)

var port = flag.Int("port", 18080, "gRPC server port")

type server struct {
	extprocv3.UnimplementedExternalProcessorServer
}

func (s *server) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	log.Println("new ext_proc stream started")
	var modelBuf []byte
	headerResponseSent := false

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("recv error: %v", err)
			return err
		}

		switch r := req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			log.Printf("received request headers (eos=%v)", r.RequestHeaders.EndOfStream)
			// Only respond to headers when there is no body (end_of_stream).
			// When a body is expected, we defer the header response until we
			// can extract the model from the body for header mutation.
			if r.RequestHeaders.EndOfStream {
				if err := stream.Send(&extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extprocv3.HeadersResponse{},
					},
				}); err != nil {
					return err
				}
				headerResponseSent = true
			}

		case *extprocv3.ProcessingRequest_RequestBody:
			log.Printf("received body chunk (eos=%v, len=%d)", r.RequestBody.EndOfStream, len(r.RequestBody.Body))

			// If we haven't sent the header response yet, accumulate body
			// data to incrementally extract the model field. Once found
			// (or at end-of-stream), send the header response immediately.
			if !headerResponseSent {
				modelBuf = append(modelBuf, r.RequestBody.Body...)
				model, found := tryExtractModel(modelBuf)
				if found || r.RequestBody.EndOfStream {
					log.Printf("extracted model: %q", model)
					var headerMutation *extprocv3.HeaderMutation
					if model != "" {
						headerMutation = &extprocv3.HeaderMutation{
							SetHeaders: []*corev3.HeaderValueOption{
								{
									Header: &corev3.HeaderValue{
										Key:      "X-Gateway-Model-Name",
										RawValue: []byte(model),
									},
								},
							},
						}
					}
					if err := stream.Send(&extprocv3.ProcessingResponse{
						Response: &extprocv3.ProcessingResponse_RequestHeaders{
							RequestHeaders: &extprocv3.HeadersResponse{
								Response: &extprocv3.CommonResponse{
									HeaderMutation: headerMutation,
								},
							},
						},
					}); err != nil {
						return err
					}
					headerResponseSent = true
					modelBuf = nil
				}
			}

			// Full-duplex: forward each body chunk immediately instead of
			// buffering the entire body. The data plane receives body data
			// as it arrives rather than waiting for end-of-stream.
			if err := stream.Send(&extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestBody{
					RequestBody: &extprocv3.BodyResponse{
						Response: &extprocv3.CommonResponse{
							BodyMutation: &extprocv3.BodyMutation{
								Mutation: &extprocv3.BodyMutation_StreamedResponse{
									StreamedResponse: &extprocv3.StreamedBodyResponse{
										Body:        r.RequestBody.Body,
										EndOfStream: r.RequestBody.EndOfStream,
									},
								},
							},
						},
					},
				},
			}); err != nil {
				return err
			}

		case *extprocv3.ProcessingRequest_ResponseHeaders:
			if err := stream.Send(&extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extprocv3.HeadersResponse{},
				},
			}); err != nil {
				return err
			}

		case *extprocv3.ProcessingRequest_ResponseBody:
			// POC: Echo the response body back via StreamedResponse so the
			// client receives it.
			if err := stream.Send(&extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseBody{
					ResponseBody: &extprocv3.BodyResponse{
						Response: &extprocv3.CommonResponse{
							BodyMutation: &extprocv3.BodyMutation{
								Mutation: &extprocv3.BodyMutation_StreamedResponse{
									StreamedResponse: &extprocv3.StreamedBodyResponse{
										Body:        r.ResponseBody.Body,
										EndOfStream: r.ResponseBody.EndOfStream,
									},
								},
							},
						},
					},
				},
			}); err != nil {
				return err
			}

		default:
			log.Printf("unknown request type: %T", r)
		}
	}
}

// tryExtractModel attempts to extract the top-level "model" field from a
// potentially incomplete JSON object using streaming token parsing.
// Returns the model string and whether extraction succeeded. When found is
// false, the caller should accumulate more data and retry.
func tryExtractModel(data []byte) (model string, found bool) {
	dec := json.NewDecoder(bytes.NewReader(data))

	// Expect opening '{'
	t, err := dec.Token()
	if err != nil {
		return "", false
	}
	delim, ok := t.(json.Delim)
	if !ok || delim != '{' {
		return "", false
	}

	for dec.More() {
		// Read key
		t, err = dec.Token()
		if err != nil {
			return "", false // incomplete data
		}
		key, ok := t.(string)
		if !ok {
			return "", false
		}

		if key == "model" {
			t, err = dec.Token()
			if err != nil {
				return "", false // value not yet received
			}
			if s, ok := t.(string); ok {
				return s, true
			}
			return "", true // model exists but is not a string
		}

		// Skip value for non-"model" keys (handles nested objects/arrays)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return "", false // incomplete data, need more chunks
		}
	}

	return "", true // all keys parsed, no model found
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(grpcServer, &server{})
	log.Printf("ext-proc BBR server listening on :%d", *port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
