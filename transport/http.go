package transport

import (
	"bytes"
	"fmt"
	"github.com/YasiruR/didcomm-prober/crypto"
	"github.com/gorilla/mux"
	"github.com/tryfix/log"
	"io/ioutil"
	"net/http"
	"strconv"
)

type HTTP struct {
	port   int
	router *mux.Router
	client *http.Client
	enc    *crypto.Encryptor
	km     *crypto.KeyManager
	logger log.Logger // remove later
}

func NewHTTP(port int, enc *crypto.Encryptor, km *crypto.KeyManager, logger log.Logger) *HTTP {
	return &HTTP{
		port:   port,
		router: mux.NewRouter(),
		client: &http.Client{},
		enc:    enc,
		km:     km,
		logger: logger,
	}
}

func (h *HTTP) Start() {
	h.router.HandleFunc(`/`, h.handleInbound).Methods(http.MethodPost)
	h.logger.Info(fmt.Sprintf("http server started listening on %d", h.port))
	if err := http.ListenAndServe(":"+strconv.Itoa(h.port), h.router); err != nil {
		h.logger.Fatal(err)
	}
}

func (h *HTTP) Send(data []byte, endpoint string) error {
	fmt.Println("ENDPOINT: ", endpoint)
	fmt.Println("DATA: ", string(data))

	res, err := h.client.Post(endpoint, `application/json`, bytes.NewBuffer(data))
	if err != nil {
		h.logger.Error(err)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusOK {
		return nil
	}

	return fmt.Errorf(`invalid status code: %d`, res.StatusCode)
}

func (h *HTTP) handleInbound(_ http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(err)
		return
	}

	text, err := h.enc.Unpack(data, h.km.PublicKey(), h.km.PrivateKey())
	if err != nil {
		return
	}

	h.logger.Debug("received msg: ", text)
}

func (h *HTTP) Stop() error {
	return nil
}
