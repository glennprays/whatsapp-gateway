package docs

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/config"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	swaggerByte []byte
	cfg         *config.Config
)

func NewSwagger(conf *config.Config) {
	cfg = conf
	swaggerByte = serveDynamicSwagger()

	if len(swaggerByte) == 0 {
		log.Warn("No Swagger documentation found, serving empty response.")
	}
	log.Info("Swagger documentation loaded successfully.")
}

func ServeDynamicSwaggerGin(c *gin.Context) {
	if len(swaggerByte) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Swagger documentation not found"})
		return
	}

	c.Header("Content-Type", "application/x-yaml")
	c.String(http.StatusOK, string(swaggerByte))
}

func serveDynamicSwagger() []byte {
	yamlFile, err := os.ReadFile("docs/swagger.yaml")
	if err != nil {
		log.Fatalf("Error reading swagger.yaml: %v", err)
	}

	var data yaml.Node
	err = yaml.Unmarshal(yamlFile, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling swagger.yaml: %v", err)
	}

	baseURL := cfg.BasePath
	if baseURL == "" {
		baseURL = "/"
		log.Println("BASE_PATH not set, using default:", baseURL)
	}

	err = updateServerURL(&data, baseURL)
	if err != nil {
		log.Fatalf("Error updating server URL in swagger.yaml: %v", err)
	}

	modifiedYAML, err := yaml.Marshal(&data)
	if err != nil {
		log.Fatalf("Error marshaling modified swagger.yaml: %v", err)
	}

	return modifiedYAML
}

func updateServerURL(node *yaml.Node, newURL string) error {
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		log.Warn("Invalid YAML document structure")
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
							log.Infof("Updated server URL to: %s", newURL)
							return nil
						}
					}
				}
			}
		}
	}
	return nil
}

func BasicAuthMiddleware() gin.HandlerFunc {
	expectedUser := cfg.SwaggerUser
	expectedPassword := cfg.SwaggerPassword

	return func(c *gin.Context) {
		user, password, hasAuth := c.Request.BasicAuth()

		if hasAuth && user == expectedUser && password == expectedPassword {
			// Credentials are valid, continue to the next handler
			c.Next()
		} else {
			// Credentials are not valid or not provided
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}
