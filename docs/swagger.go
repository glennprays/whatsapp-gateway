package docs

import (
	"net/http"
	"os"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

var (
	swaggerByte []byte
	cfg         *config.Config
	logger      *log.Logger
	traceID     string
)

func NewSwagger(trcID string, conf *config.Config, logg *log.Logger) {
	logger = logg
	cfg = conf
	traceID = trcID
	swaggerByte = serveDynamicSwagger()

	if len(swaggerByte) == 0 {
		logger.Warn(traceID, "No Swagger documentation found, serving empty response", nil)
	}
	logger.Info(traceID, "Swagger documentation loaded successfully.", nil)
}

func ServeDynamicSwaggerFiber(c *fiber.Ctx) error {
	if len(swaggerByte) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "Swagger documentation not found"})
	}

	c.Set("Content-Type", "application/x-yaml")
	return c.Status(http.StatusOK).SendString(string(swaggerByte))
}

func serveDynamicSwagger() []byte {
	yamlFile, err := os.ReadFile("docs/swagger.yaml")
	if err != nil {
		logger.Fatal(traceID, "Error reading swagger.yaml", log.Error(err))
	}

	var data yaml.Node
	err = yaml.Unmarshal(yamlFile, &data)
	if err != nil {
		logger.Fatal(traceID, "Error unmarshaling swagger.yaml", log.Error(err))
	}

	baseURL := cfg.BasePath
	if baseURL == "" {
		baseURL = "/"
		logger.Info(traceID, "BASE_PATH not set, using default", log.String("basePath", baseURL))
	}

	err = updateServerURL(&data, baseURL)
	if err != nil {
		logger.Fatal(traceID, "Error updating server URL in swagger.yaml", log.Error(err))
	}

	modifiedYAML, err := yaml.Marshal(&data)
	if err != nil {
		logger.Fatal(traceID, "Error marshaling modified swagger.yaml", log.Error(err))
	}

	return modifiedYAML
}

func updateServerURL(node *yaml.Node, newURL string) error {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		logger.Warn(traceID, "Invalid YAML document structure", nil)
		return nil
	}
	root := node.Content[0]

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		if keyNode.Value == "servers" {
			serversNode := root.Content[i+1]
			if serversNode.Kind == yaml.SequenceNode && len(serversNode.Content) > 0 {
				firstServerNode := serversNode.Content[0]
				if firstServerNode.Kind == yaml.MappingNode {
					for j := 0; j < len(firstServerNode.Content); j += 2 {
						if firstServerNode.Content[j].Value == "url" {
							firstServerNode.Content[j+1].Value = newURL
							logger.Info(traceID, "Updated server URL in Swagger documentation", map[string]any{
								"new_url": newURL,
							})
							return nil
						}
					}
				}
			}
		}
	}
	return nil
}
