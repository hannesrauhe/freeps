package utils

import "github.com/google/uuid"

type Context struct {
	uuid uuid.UUID
}

func NewContext() *Context {
	return &Context{uuid: uuid.New()}
}

func (c *Context) GetID() string {
	return c.uuid.String()
}
