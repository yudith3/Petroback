package config

// This package is intentionally kept configuration-only.
// Any HTTP/bootstrap wiring should live in app/rest.go so we do not create a circular
// dependency between config and db packages.
