package docs

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func ServeDynamicSwagger(c *gin.Context) {
	yamlFile, err := os.ReadFile("docs/swagger.yaml")
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not read swagger.yaml: %v", err)
		log.Warnf("Error reading swagger.yaml: %v", err)
		return
	}

	var data yaml.Node
	err = yaml.Unmarshal(yamlFile, &data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not parse swagger.yaml: %v", err)
		log.Warnf("Error unmarshaling swagger.yaml: %v", err)
		return
	}

	baseURL := os.Getenv("BASE_PATH")
	if baseURL == "" {
		baseURL = "/"
		log.Println("BASE_PATH not set, using default:", baseURL)
	}

	err = updateServerURL(&data, baseURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not update swagger.yaml content: %v", err)
		log.Warnf("Error updating server URL: %v", err)
		return
	}

	modifiedYAML, err := yaml.Marshal(&data)
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not generate modified swagger.yaml: %v", err)
		log.Warnf("Error marshaling modified YAML: %v", err)
		return
	}

	c.Data(http.StatusOK, "application/x-yaml", modifiedYAML)
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
	expectedUser := os.Getenv("SWAGGER_USER")
	expectedPassword := os.Getenv("SWAGGER_PASSWORD")

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
