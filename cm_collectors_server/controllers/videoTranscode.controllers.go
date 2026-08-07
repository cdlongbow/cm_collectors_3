package controllers

import (
	"cm_collectors_server/processors"
	"cm_collectors_server/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type VideoTranscode struct{}

func (VideoTranscode) List(c *gin.Context) {
	data, err := (processors.VideoTranscode{}).List()
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(data, c)
}

func (VideoTranscode) Capabilities(c *gin.Context) {
	response.OkWithData((processors.VideoTranscode{}).Capabilities(), c)
}

func (VideoTranscode) Add(c *gin.Context) {
	var request processors.VideoTranscodeAddRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	data, err := (processors.VideoTranscode{}).Add(request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(data, c)
}

func (VideoTranscode) UpdateConfig(c *gin.Context) {
	var request processors.VideoTranscodeUpdateConfigRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	if err := (processors.VideoTranscode{}).UpdateConfig(request); ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (VideoTranscode) SaveEditPlan(c *gin.Context) {
	var request processors.VideoTranscodeEditPlanRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	result, err := (processors.VideoTranscode{}).SaveEditPlan(c.Param("id"), request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(result, c)
}

func (VideoTranscode) Thumbnail(c *gin.Context) {
	at, err := strconv.ParseFloat(c.Query("at"), 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "缩略图时间点无效"})
		return
	}
	data, err := (processors.VideoTranscode{}).Thumbnail(c.Request.Context(), c.Param("id"), at)
	if err := ResError(c, err); err != nil {
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, "image/jpeg", data)
}

func (VideoTranscode) ThumbnailBatch(c *gin.Context) {
	var request processors.VideoTranscodeThumbnailBatchRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	data, err := (processors.VideoTranscode{}).ThumbnailBatch(c.Request.Context(), c.Param("id"), request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(data, c)
}

func (VideoTranscode) TransitionPreview(c *gin.Context) {
	var request processors.VideoTranscodeTransitionPreviewRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	path, err := (processors.VideoTranscode{}).TransitionPreview(c.Request.Context(), c.Param("id"), request)
	if err := ResError(c, err); err != nil {
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("Content-Type", "video/webm")
	c.File(path)
}

func (VideoTranscode) Start(c *gin.Context) {
	var request processors.VideoTranscodeStartRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	result, err := (processors.VideoTranscode{}).Start(request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(result, c)
}

func (VideoTranscode) ResetBatch(c *gin.Context) {
	var request processors.VideoTranscodeIDsRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	result, err := (processors.VideoTranscode{}).ResetBatch(request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(result, c)
}

func (VideoTranscode) RetryReplacement(c *gin.Context) {
	if err := (processors.VideoTranscode{}).RetryReplacement(c.Param("id")); ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (VideoTranscode) SaveVerifiedOutputAsNewFile(c *gin.Context) {
	if err := (processors.VideoTranscode{}).SaveVerifiedOutputAsNewFile(c.Param("id")); ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (VideoTranscode) Pause(c *gin.Context) {
	(processors.VideoTranscode{}).Pause()
	response.OkWithData(true, c)
}

func (VideoTranscode) Resume(c *gin.Context) {
	(processors.VideoTranscode{}).Resume()
	response.OkWithData(true, c)
}

func (VideoTranscode) QueueStatus(c *gin.Context) {
	response.OkWithData((processors.VideoTranscode{}).QueueStatus(), c)
}

func (VideoTranscode) Cancel(c *gin.Context) {
	if err := (processors.VideoTranscode{}).Cancel(c.Param("id")); ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (VideoTranscode) Delete(c *gin.Context) {
	if err := (processors.VideoTranscode{}).Delete(c.Param("id")); ResError(c, err) != nil {
		return
	}
	response.OkWithData(true, c)
}

func (VideoTranscode) DeleteBatch(c *gin.Context) {
	var request processors.VideoTranscodeIDsRequest
	if err := ParameterHandleShouldBindJSON(c, &request); err != nil {
		return
	}
	count, err := (processors.VideoTranscode{}).DeleteBatch(request)
	if err := ResError(c, err); err != nil {
		return
	}
	response.OkWithData(count, c)
}
