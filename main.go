package main

import (
	"fmt"
	rice "github.com/GeertJohan/go.rice"
	"github.com/pflow-dev/go-metamodel/v2/server"
	"github.com/pflow-dev/pflow-cli/app"
	"github.com/pflow-dev/pflow-cli/internal/examples"
	"github.com/pflow-dev/pflow-cli/storage"
	"net/http"
	"os"
)

var (
	options = app.Options{
		Host:         "127.0.0.1",
		Port:         "8083",
		Url:          "http://localhost:8083",
		DbPath:       "/tmp/pflow.db",
		LoadExamples: true,
		UseSandbox:   false, // sandbox relies on js from CDN
	}
)

func main() {
	dbPath, pathSet := os.LookupEnv("DB_PATH")
	if pathSet {
		options.DbPath = dbPath
	}
	baseUrl, urlSet := os.LookupEnv("URL_BASE")
	if urlSet {
		options.Url = baseUrl
	}
	listenPort, portSet := os.LookupEnv("PORT")
	if portSet {
		options.Port = listenPort
	}
	listenHost, hostSet := os.LookupEnv("HOST")
	if hostSet {
		options.Host = listenHost
	}
	_, sandboxSet := os.LookupEnv("USE_SANDBOX")
	if sandboxSet {
		options.UseSandbox = true
	}
	store := storage.New(storage.ResetDb(options.DbPath))

	s := app.New(server.Storage{
		Model:   store.Model,
		Snippet: store.Snippet,
	}, options)

	if options.LoadExamples {
		for _, m := range examples.ExampleModels {
			_, _ = store.Model.Create(m.IpfsCid, m.Base64Zipped, m.Title, m.Description, m.Keywords, "http://localhost:8083/p/")
			foundModel := store.Model.GetByCid(m.IpfsCid)
			if foundModel.IpfsCid != m.IpfsCid {
				panic(fmt.Sprintf("Failed to load model %s %s", m.Title, m.IpfsCid))
			}
			s.PrintLinks(foundModel.ToModel(), options.Url)
		}
		s.Logger.Print("Loaded example models")
	}
	box := rice.MustFindBox("./public")
	s.ServeHTTP(http.FileServer(box.HTTPBox()))
}
