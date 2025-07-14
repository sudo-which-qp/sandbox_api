# QP API Sandbox

A flexible REST API sandbox built with Go for testing and prototyping common backend operations including authentication, authorization, CRUD operations, and other API patterns.

## Features

- **Authentication & Authorization**: JWT-based auth, role-based access control
- **CRUD Operations**: Complete Create, Read, Update, Delete functionality
- **Database Integration**: Support for multiple database backends
- **Middleware**: Request logging, CORS, rate limiting
- **Testing Environment**: Perfect for API testing and development
- **Docker Support**: Easy containerization and deployment

## Prerequisites

- Go 1.24 or higher
- Database (PostgreSQL or MySQL)
- Docker

## Installation

1. Clone the repository:
```bash
git clone https://github.com/sudo-which-qp/sandbox_api
cd sandbox_api
```

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Run database migrations:
```bash
make migrate-up
```

## Usage

### Starting the Server
There are two docker files available: `Dockerfile` and `Dockerfile.dev`. 
On the docker-compose.yml you can change it there for dev or production. 
But you can also run the server with the go command: `go run cmd/api/main.go`. 
if you don't have docker installed. I will recommend you to using `Air` if you want to use docker.
As `Air` is already installed in the project, you can run the server with the `air` command.

```bash
# Development mode
docker-compose up --build
```

## API Endpoints

### Authentication
- `POST /v1/auth/register` - Register a new user
- `POST /v1/auth/login` - Login user
- `POST /v1/auth/verify-email` - Verify user email
- `POST /v1/auth/forgot-password` - Forgot password
- `POST /v1/auth/reset-password` - Reset password
- `POST /v1/auth/resend-otp` - Resend OTP

### Example API Calls

```bash
# Register a new user
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
  "first_name":"Test",
  "last_name":"User",
  "username":"testuser",
  "email":"test@example.com",
  "password":"password123"
  }'

# Login
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```


### User Management
- `GET /v1/user/profile` - Get user profile
- `POST /v1/user/update-profile` - Update user profile

### Example API Calls

```bash
# Update user profile
curl -X POST http://localhost:8080/v1/user/update-profile \
  -H "Content-Type: application/json, Authorization: Bearer <token>" \
  -d '{
  "first_name":"Test",
  "last_name":"User",
  }'

# Get user profile
curl -X GET http://localhost:8080/v1/user/profile \
  -H "Content-Type: application/json, Authorization: Bearer <token>"
```


## Development

### Adding New Endpoints

1. Define the model in `internal/models/`
2. Create database repository `internal/store/`
3. Add HTTP handlers in `cmd/api/`
4. Register routes in the main server file

### Database Migrations

```bash
# Create new migration
make migration-create user_table

# Run migrations
make migrate-up

# Rollback migrations
make migrate-down
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the CC0 1.0 Universal License - see the [LICENSE](LICENSE) file for details.