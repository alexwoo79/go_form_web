# go-web-web-forms

Repository for Go Web Form System

## Project Structure

```
go-web/
├── README.md
├── QUICKSTART.md
├── LICENSE
├── go.mod
├── config.yaml
├── config.example.yaml
├── build.sh
├── init.sh
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── generate/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── router.go
│   │   └── config_test.go
│   ├── handler/
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   └── mock_test.go
│   ├── models/
│   │   ├── database.go
│   │   └── database_test.go
│   └── utils/
│       ├── form_generator.go
│       └── form_generator_test.go
├── ui/
│   └── templates/
│       ├── index.html
│       └── form.html
├── data/
├── generated/
└── bin/
```

## Key Features

- YAML form configuration
- SQLite database storage
- Form validation
- File-based data backup
- Responsive UI
- RESTful API endpoints
