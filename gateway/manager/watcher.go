package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/go-openapi/spec"
	"github.com/graphql-go/graphql"
	"path/filepath"

	"github.com/openmfp/golang-commons/sentry"

	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

var (
	ErrUnknownFileEvent = errors.New("unknown file event")
)

type FileWatcher interface {
	OnFileChanged(filename string)
	OnFileDeleted(filename string)
}

func (s *Service) Start() {
	go func() {
		for {
			select {
			case event, ok := <-s.watcher.Events:
				if !ok {
					return
				}
				s.handleEvent(event)
			case err, ok := <-s.watcher.Errors:
				if !ok {
					return
				}
				s.log.Error().Err(err).Msg("Error watching files")
				sentry.CaptureError(err, nil)
			}
		}
	}()
}

func (s *Service) handleEvent(event fsnotify.Event) {
	s.log.Info().Str("event", event.String()).Msg("File event")

	filename := filepath.Base(event.Name)
	switch event.Op {
	case fsnotify.Create:
		s.OnFileChanged(filename)
	case fsnotify.Write:
		s.OnFileChanged(filename)
	case fsnotify.Rename:
		s.OnFileDeleted(filename)
	case fsnotify.Remove:
		s.OnFileDeleted(filename)
	default:
		err := ErrUnknownFileEvent
		s.log.Error().Err(err).Str("filename", filename).Msg("Unknown file event")
		sentry.CaptureError(sentry.SentryError(err), nil, sentry.Extras{"filename": filename, "event": event.String()})
	}
}

func (s *Service) OnFileChanged(filename string) {
	schema, err := s.loadSchemaFromFile(filename)
	if err != nil {
		s.log.Error().Err(err).Str("filename", filename).Msg("failed to process the file's change")
		sentry.CaptureError(err, sentry.Tags{"filename": filename})

		return
	}

	s.handlers.mu.Lock()
	s.handlers.registry[filename] = s.createHandler(schema)
	s.handlers.mu.Unlock()

	s.log.Info().Str("endpoint", fmt.Sprintf("http://localhost:%s/%s/graphql", s.AppCfg.Gateway.Port, filename)).Msg("Registered endpoint")
}

func (s *Service) OnFileDeleted(filename string) {
	s.handlers.mu.Lock()
	defer s.handlers.mu.Unlock()

	delete(s.handlers.registry, filename)
}

func (s *Service) loadSchemaFromFile(filename string) (*graphql.Schema, error) {
	definitions, err := ReadDefinitionFromFile(filepath.Join(s.AppCfg.OpenApiDefinitionsPath, filename))
	if err != nil {
		return nil, err
	}

	g, err := schema.New(s.log, definitions, s.resolver)
	if err != nil {
		return nil, err
	}

	return g.GetSchema(), nil
}

func ReadDefinitionFromFile(filePath string) (spec.Definitions, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var swagger spec.Swagger
	err = json.NewDecoder(f).Decode(&swagger)
	if err != nil {
		return nil, err
	}

	return swagger.Definitions, nil
}
