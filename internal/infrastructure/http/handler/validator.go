package handler

import "github.com/go-playground/validator/v10"

type RequestValidator struct { validator *validator.Validate }

func NewRequestValidator() *RequestValidator { return &RequestValidator{validator: validator.New()} }

func (rv *RequestValidator) Validate(i interface{}) error { return rv.validator.Struct(i) }
