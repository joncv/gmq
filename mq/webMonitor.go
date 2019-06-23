package mq

import (
	"fmt"
	"go-mq/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WebMonitor struct {
}

var Wmor *WebMonitor

func init() {
	Wmor = &WebMonitor{}
}

func (w *WebMonitor) Run() {
	// defer gmq.wg.Done()
	// gmq.wg.Add(1)

	if gmq.running == 0 {
		return
	}

	r := gin.Default()
	r.StaticFS("/static", http.Dir("static"))
	r.LoadHTMLGlob("views/*")
	r.GET("/", w.index)
	r.GET("/login", w.login)
	r.GET("/home", w.home)
	r.GET("/bucketList", w.bucketList)
	r.GET("/bucketJobList", w.bucketJobList)
	r.GET("/readyQueueList", w.readyQueueList)
	r.GET("/getReadyQueueStat", w.getReadyQueueStat)
	r.GET("/getBucketStat", w.getBucketStat)
	r.GET("/getJobsByBucketKey", w.getJobsByBucketKey)
	r.GET("/jobDetail", w.jobDetail)
	r.GET("/test", w.test)
	r.Run(":8000")
}

// 首页
func (w *WebMonitor) index(c *gin.Context) {
	c.HTML(http.StatusOK, "entry.html", gin.H{
		"siteName":      "web监控管理",
		"version":       "v1.0",
		"loginUserName": "wuzhc",
	})
}

// 主页
func (w *WebMonitor) home(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"title": "主页",
	})
}

// 登录
func (w *WebMonitor) login(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录页面",
	})
}

// bucket列表页
func (w *WebMonitor) bucketList(c *gin.Context) {
	c.HTML(http.StatusOK, "bucket_list.html", gin.H{
		"title": "bucket列表",
	})
}

// bucket中job列表
func (w *WebMonitor) bucketJobList(c *gin.Context) {
	bucketKey := c.Query("bucketKey")
	if len(bucketKey) == 0 {
		c.String(http.StatusBadRequest, "bucketKey参数错误")
		return
	}
	c.HTML(http.StatusOK, "bucket_job_list.html", gin.H{
		"title":     "bucket jobs列表",
		"bucketKey": bucketKey,
	})
}

// ready queue列表页
func (w *WebMonitor) readyQueueList(c *gin.Context) {
	c.HTML(http.StatusOK, "readyqueue_list.html", gin.H{
		"title": "readyQueue列表",
	})
}

// 任务job详情
func (w *WebMonitor) jobDetail(c *gin.Context) {
	jobId := c.Query("jobId")
	if len(jobId) == 0 {
		c.String(http.StatusBadGateway, "jobId参数错误")
		return
	}
	detail, err := GetJobDetailById(jobId)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	detail["delay"] = utils.SecToTimeString(detail["delay"])
	detail["job_key"] = GetJobKeyById(detail["id"])
	c.HTML(http.StatusOK, "job_detail.html", gin.H{
		"title":  "job详情",
		"detail": detail,
	})
}

// 根据jobId获取job详情
func (w *WebMonitor) getJobDetailById(c *gin.Context) {
	jobId := c.Query("jobId")
	if len(jobId) == 0 {
		c.JSON(http.StatusBadRequest, w.rspErr("jobId参数错误"))
		return
	}
	detail, err := GetJobDetailById(jobId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, w.rspErr(err))
		return
	}
	c.JSON(http.StatusOK, detail)
}

// 根据bucketKey获取bucket任务列表
func (w *WebMonitor) getJobsByBucketKey(c *gin.Context) {
	n := c.DefaultQuery("limit", "20")
	k := c.Query("bucketKey")
	if len(k) == 0 {
		c.JSON(http.StatusBadRequest, w.rspErr("bucketKey不能为空"))
		return
	}

	type jobInfo struct {
		Id        string `json:"id"`
		JobKey    string `json:"job_key"`
		RunTime   string `json:"runtime"`
		TTR       string `json:"ttr"`
		DelayTime string `json:"delay_time"`
		Topic     string `json:"topic"`
		Status    string `json:"status"`
	}
	var res []jobInfo
	records, err := Redis.Strings("ZRANGE", k, 0, n, "WITHSCORES")
	if err != nil {
		c.JSON(http.StatusInternalServerError, w.rspErr(err))
		return
	}

	var name, time []string
	for i, v := range records {
		if i%2 == 0 {
			name = append(name, v)
		} else {
			time = append(time, v)
		}
	}

	for j, id := range name {
		detail, _ := GetJobDetailById(id)
		res = append(res, jobInfo{
			Id:        id,
			TTR:       detail["TTR"],
			DelayTime: utils.SecToTimeString(detail["delay"]),
			Topic:     detail["topic"],
			Status:    w.getStatusName(detail["status"]),
			JobKey:    GetJobKeyById(id),
			RunTime:   utils.UnixToFormatTime(time[j]),
		})
	}

	c.JSON(http.StatusOK, w.rspData(res))
}

func (w *WebMonitor) getStatusName(status string) string {
	s, err := strconv.Atoi(status)
	if err != nil {
		return `<span class="layui-badge layui-bg-black">unknown</span>`
	}
	if s == JOB_STATUS_DELAY {
		return `<span class="layui-badge layui-bg-orange">delay</span>`
	}
	if s == JOB_STATUS_READY {
		return `<span class="layui-badge layui-bg-cyan">ready</span>`
	}
	if s == JOB_STATUS_RESERVED {
		return `<span class="layui-badge layui-bg-blue">reserved</span>`
	}
	return `<span class="layui-badge layui-bg-black">unknown</span>`
}

// 获取readyQueue统计信息
func (w *WebMonitor) getReadyQueueStat(c *gin.Context) {
	records, err := Redis.Strings("KEYS", READY_QUEUE_KEY+"*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, w.rspErr(err))
	}

	type queueInfo struct {
		Id        int    `json:"id"`
		QueueName string `json:"queue_name"`
		JobNum    int    `json:"job_num"`
	}
	var res []queueInfo
	for k, r := range records {
		num, err := Redis.Int("LLEN", r)
		if err != nil {
			num = 0
		}
		res = append(res, queueInfo{
			Id:        k + 1,
			QueueName: r,
			JobNum:    num,
		})
	}

	c.JSON(http.StatusOK, w.rspData(res))
}

// 获取bucket统计信息
func (w *WebMonitor) getBucketStat(c *gin.Context) {
	type bucketInfo struct {
		Id         int    `json:"id"`
		BucketName string `json:"bucket_name"`
		JobNum     int    `json:"job_num"`
		NextTime   string `json:"next_time"`
	}
	var res []bucketInfo
	for k, b := range Dper.bucket {
		res = append(res, bucketInfo{
			Id:         k + 1,
			BucketName: GetBucketKeyById(b.Id),
			JobNum:     b.JobNum,
			NextTime:   utils.FormatTime(b.NextTime),
		})
	}

	c.JSON(http.StatusOK, w.rspData(res))
}

func (w *WebMonitor) rspErr(msg interface{}) gin.H {
	var resp = make(gin.H)
	resp["code"] = 1
	resp["msg"] = msg
	resp["data"] = nil
	return resp
}

func (w *WebMonitor) rspData(data interface{}) gin.H {
	var resp = make(gin.H)
	resp["code"] = 0
	resp["msg"] = ""
	resp["data"] = data
	return resp
}

func (w *WebMonitor) rspSuccess(msg interface{}) gin.H {
	var resp = make(gin.H)
	resp["code"] = 0
	resp["msg"] = msg
	resp["data"] = nil
	return resp
}

func (w *WebMonitor) test(c *gin.Context) {
	res, err := GetJobStatus("xxxxe")
	if err != nil {
		fmt.Println("error", err)
	} else {
		fmt.Println(res, err)
	}
}