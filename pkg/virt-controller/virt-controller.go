package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"golang.org/x/net/context"
	"log"
	"net/http"

	"flag"
	"github.com/go-kit/kit/endpoint"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"strconv"
)

const DefaultMaxContentLengthBytes = 3 << 20

type VMService interface {
	StartVMRaw(string, []byte) error
}

type vmService struct{}

func (v *vmService) StartVMRaw(name string, rawXML []byte) error {
	return nil
}

type Handlers struct {
	RawDomainHandler http.Handler
}

func main() {

	host := flag.String("address", "0.0.0.0", "Address to bind to")
	port := flag.Int("port", 8080, "Port to listen on")

	flag.Parse()
	ctx := context.Background()
	svc := vmService{}

	endpoints := Handlers{
		RawDomainHandler: makeRawDomainHandler(ctx, makeRawDomainEndpoint(&svc)),
	}

	http.Handle("/", defineRoutes(&endpoints))
	log.Printf("Listening on %s:%d", *host, *port)
	log.Fatal(http.ListenAndServe(*host+":"+strconv.Itoa(*port), nil))
}

func defineRoutes(endpoints *Handlers) http.Handler {
	router := mux.NewRouter()
	restV1Route := router.PathPrefix("/api/v1/").Subrouter()
	restV1Route.Methods("POST").Path("/domain/raw").Headers("Content-Type", "application/xml").Handler(endpoints.RawDomainHandler)
	return router
}

func makeRawDomainEndpoint(svc VMService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(VMRequestDTO)
		err := svc.StartVMRaw(req.Name, req.RawDomain)
		if err != nil {
			return ResponseDTO{Err: err.Error()}, nil
		}
		return ResponseDTO{Err: ""}, nil
	}
}

func makeRawDomainHandler(ctx context.Context, endpoint endpoint.Endpoint) http.Handler {
	return kithttp.NewServer(
		ctx,
		endpoint,
		decodeRawDomainRequest,
		encodeResponse,
	)
}

func decodeRawDomainRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var vm VMRequestDTO
	var body []byte
	body, err := checkAndExtractBody(r.Body, DefaultMaxContentLengthBytes)
	if err != nil {
		return nil, err
	}

	if err := xml.Unmarshal(body, &vm); err != nil {
		return nil, err
	}
	if vm.Name == "" {
		return nil, errors.New("VM name is missing")
	}
	vm.RawDomain = body
	return vm, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

type VMRequestDTO struct {
	XMLName   xml.Name `xml:"domain"`
	Name      string   `xml:"name"`
	RawDomain []byte
}

type ResponseDTO struct {
	Err string `json:"err,omitempty"`
}

// TODO make this usable as a wrapping handler func or replace with http.MaxBytesReader
func checkAndExtractBody(http_body io.ReadCloser, maxContentLength int64) ([]byte, error) {
	body, err := ioutil.ReadAll(io.LimitReader(http_body, maxContentLength+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxContentLength {
		return nil, errors.New("http: POST too large")
	}
	return body, nil
}
