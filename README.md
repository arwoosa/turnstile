# Cloudflare Turnstile Plugin for Traefik

A Traefik middleware plugin that integrates Cloudflare's Turnstile CAPTCHA service to protect your web applications from automated attacks. This plugin allows you to selectively protect specific routes with Turnstile verification.

![Cloudflare Turnstile](.assets/logo.png)

## Features

- 🛡️ Protect specific routes with Cloudflare Turnstile verification
- 🎯 Selective route protection based on HTTP method and path
- 🔒 Secure token verification through Cloudflare's API
- 🚦 Comprehensive error handling and reporting
- 🔄 Maintains request integrity during verification
- 📦 Support for path parameters in route matching

## Prerequisites

- Traefik v2.x
- A Cloudflare account with Turnstile enabled
- Your Cloudflare Turnstile secret key

## Installation

To install this plugin, you need to declare it in your Traefik static configuration. Here's how to do it:

```yaml
experimental:
  plugins:
    turnstile:
      moduleName: "github.com/arwoosa/turnstile"
      version: "v1.0.0"  # Use the latest version
```

## Configuration

### Static Configuration

The plugin can be configured using the following options in your Traefik dynamic configuration:

```yaml
http:
  middlewares:
    my-turnstile:
      plugin:
        turnstile:
          turnstilesecret: "your-turnstile-secret-key"
          routers:
            - method: POST
              path: /verify
              headerkey: "X-Turnstile-Token"  # Optional: specify header key
              formkey: "cf-turnstile-response"  # Optional: specify form key
            - method: GET
              path: /api/identity/{token}  # Support for path parameters
              headerkey: "X-Turnstile-Token"
```

### Configuration Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `turnstilesecret` | String | Yes | Your Cloudflare Turnstile secret key |
| `routers` | Array | Yes | List of routes to protect |
| `routers[].method` | String | Yes | HTTP method (GET, POST, etc.) |
| `routers[].path` | String | Yes | URL path to protect (supports {parameter} syntax) |
| `routers[].headerkey` | String | No | Header key to extract token from (default: none) |
| `routers[].formkey` | String | No | Form key to extract token from (default: "cf-turnstile-response") |

## Token Extraction Configuration

The plugin supports flexible token extraction from both HTTP headers and form fields. You can configure this per route using `headerkey` and `formkey` options.

### Header-based Token Extraction

To extract the token from an HTTP header:

```yaml
routers:
  - method: GET
    path: /api/identity/{token}
    headerkey: "X-Turnstile-Token"  # Token will be read from this header
```

Example request:
```http
GET /api/identity/123
X-Turnstile-Token: your-turnstile-token
```

### Form-based Token Extraction

To extract the token from a form field:

```yaml
routers:
  - method: POST
    path: /verify
    formkey: "cf-turnstile-response"  # Token will be read from this form field
```

Example request:
```http
POST /verify
Content-Type: application/x-www-form-urlencoded

cf-turnstile-response=your-turnstile-token
```

### Default Behavior

- If neither `headerkey` nor `formkey` is specified, the plugin will:
  - First try to read from the default form field `cf-turnstile-response`
  - If not found in form, return an error

### Best Practices

1. For API endpoints, prefer `headerkey`:
   ```yaml
   routers:
     - method: GET
       path: /api/endpoint
       headerkey: "X-Turnstile-Token"
   ```

2. For form submissions, use `formkey`:
   ```yaml
   routers:
     - method: POST
       path: /submit
       formkey: "cf-turnstile-response"
   ```

3. For mixed usage, specify both:
   ```yaml
   routers:
     - method: POST
       path: /api/submit
       headerkey: "X-Turnstile-Token"
       formkey: "cf-turnstile-response"
   ```

## Usage

### 1. Frontend Integration

First, add the Turnstile widget to your HTML form:

```html
<form action="/verify" method="POST">
    <div class="cf-turnstile" data-sitekey="your-site-key"></div>
    <!-- Your form fields -->
    <button type="submit">Submit</button>
</form>

<!-- Add Turnstile script -->
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
```

### 2. Traefik Configuration

Configure the middleware in your Traefik dynamic configuration:

```yaml
http:
  routers:
    my-router:
      rule: "Host(`example.com`)"
      service: "my-service"
      middlewares:
        - "my-turnstile"

  middlewares:
    my-turnstile:
      plugin:
        turnstile:
          turnstilesecret: "${TURNSTILE_SECRET}"
          routers:
            - method: POST
              path: /verify
              formkey: "cf-turnstile-response"
            - method: GET
              path: /api/identity/{token}
              headerkey: "X-Turnstile-Token"
```

## Path Parameter Support

The plugin supports path parameters in route matching. For example:

```yaml
routers:
  - method: GET
    path: /api/identity/{token}  # Will match /api/identity/123, /api/identity/abc, etc.
    headerkey: "X-Turnstile-Token"
```

The path parameter matching:
- Supports any value in place of `{parameter}`
- Is case-insensitive
- Requires exact path segment matching
- Works with multiple parameters in the same path

## How It Works

1. When a request is made to a protected route, the plugin checks for the presence of a Turnstile token
2. The token is extracted from either the specified header or form field
3. The plugin verifies the token with Cloudflare's verification API
4. If verification succeeds, the request proceeds to the next handler
5. If verification fails, an error response is returned

## Error Handling

The plugin provides detailed error responses in JSON format:

```json
{
    "error": "Error message description"
}
```

Common error scenarios:
- Missing token
- Invalid token
- Verification API errors
- Configuration errors

## Development

### Prerequisites

- Go 1.x
- Traefik v2.x

### Building

```bash
go build ./...
```

### Testing

```bash
go test ./...
```

## Security Considerations

- Keep your Turnstile secret key secure and never expose it in client-side code
- Use environment variables for sensitive configuration
- Regularly rotate your Turnstile keys
- Monitor your Turnstile analytics in Cloudflare dashboard

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Cloudflare Turnstile team for providing the CAPTCHA service
- Traefik team for the plugin system