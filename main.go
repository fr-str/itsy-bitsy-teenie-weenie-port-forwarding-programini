package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	_ "net/http/pprof"

	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/dns"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/front"
	"github.com/fr-str/itsy-bitsy-teenie-weenie-port-forwarder-programini/kube"
	"go.uber.org/zap"
)

var log *zap.SugaredLogger

func PrettyJSONString(str string) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		log.Error(err)
		return ""
	}
	return prettyJSON.String()
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Fatal: Please provide a kubeconfig name, does't have to be fullpath\nExample: fullpath:'$HOME/.kube/config', you can just type 'config'")
		os.Exit(1)
	}
	logger := initLogger()
	zap.ReplaceGlobals(logger)
	log = logger.Sugar()
	go kube.Connect(os.Args[1])
	go dns.Start()

	front.Start()
}
