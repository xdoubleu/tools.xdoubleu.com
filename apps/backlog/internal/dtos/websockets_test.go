package dtos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
)

func TestSubscribeMessageDtoTopic(t *testing.T) {
	dto := dtos.SubscribeMessageDto{Subject: "backlog"}
	assert.Equal(t, "backlog", dto.Topic())
}

func TestSubscribeMessageDtoValidate(t *testing.T) {
	dto := dtos.SubscribeMessageDto{Subject: "backlog"}
	ok, errs := dto.Validate()
	assert.True(t, ok)
	assert.Empty(t, errs)
}
