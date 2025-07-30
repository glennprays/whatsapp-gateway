window.onload = function () {
  //<editor-fold desc="Changeable Configuration Block">

  // Dynamically compute the URL to swagger.yaml based on the current path
  const currentPath = window.location.pathname;
  // Remove the trailing slash if any, and append 'swagger.yaml'
  const swaggerYamlUrl = currentPath.replace('/ui/', '/') + 'yaml'

  window.ui = SwaggerUIBundle({
    url: swaggerYamlUrl,
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  });

  //</editor-fold>
};
