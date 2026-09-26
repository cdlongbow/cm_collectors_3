package controllers

import (
	"cm_collectors_server/core"
	"cm_collectors_server/models"
	"cm_collectors_server/processors"
	"cm_collectors_server/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SharedConfig struct{}

func (SharedConfig) Status(c *gin.Context) {
	state, err := models.SharedConfigStatus(core.DBS(), c.Param("id"), c.Param("module"))
	if ResError(c, err) != nil {
		return
	}
	response.OkWithData(state, c)
}

func (SharedConfig) Save(c *gin.Context) {
	var par struct {
		Config   string `json:"config"`
		Revision int    `json:"revision"`
	}
	if ParameterHandleShouldBindJSON(c, &par) != nil {
		return
	}
	err := processors.SaveSharedLibraryConfig(c.Param("module"), par.Revision, par.Config)
	if ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (SharedConfig) Follow(c *gin.Context) {
	var par struct {
		Following  bool   `json:"following"`
		DetachMode string `json:"detachMode"`
		Revision   int    `json:"revision"`
	}
	if ParameterHandleShouldBindJSON(c, &par) != nil {
		return
	}
	err := core.DBS().Transaction(func(tx *gorm.DB) error {
		return processors.FollowSharedLibraryConfig(tx, c.Param("id"), c.Param("module"), par.Following, par.Revision, par.DetachMode)
	})
	if ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}
