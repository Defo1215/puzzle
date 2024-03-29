package services

import (
	"encoding/json"
	"errors"
	"puzzle/app/middlewares/rabbitmq"
	"puzzle/app/middlewares/rabbitmq/handlers"
	"puzzle/app/models"

	"puzzle/database"
	"puzzle/utils"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set"
	"gorm.io/gorm"
)

type RecordBestAverageService interface {
	check(record *models.RecordBestAverage) error
	Insert(record *models.RecordBestAverage) error
	List(recordReq *models.RecordBestAverageReq) (models.RecordBestAverageListResp, error)
	Update(record *models.RecordBestAverage) error
	publishMessage(rankUpdate handlers.RankUpdate)
}

type RecordBestAverageImpl struct{}

// publishMessage 发送消息至消息队列
func (RecordBestAverageImpl) publishMessage(rankUpdate handlers.RankUpdate) {
	mq := rabbitmq.NewRabbitMQ("best_average_rank_update_queue", "", "")
	defer mq.Destory()

	message := rabbitmq.RabbitMQMessage{
		RankUpdate: rankUpdate,
		Message:    "rank update",
	}

	messageByte, err := json.Marshal(message)
	if err != nil {
		return
	}

	mq.Publish(messageByte)
}

// check 校验参数
func (RecordBestAverageImpl) check(record *models.RecordBestAverage) error {
	if record.UserId == 0 {
		return errors.New("用户ID不能为空")
	}

	if record.Dimension == 0 {
		return errors.New("阶数不能为空")
	}

	if record.Type == 0 {
		return errors.New("类型不能为空")
	}

	if record.RecordIds == "" {
		return errors.New("记录ID不能为空")
	}

	if record.RecordAverageDuration == 0 {
		return errors.New("平均时长不能为空")
	}

	return nil
}

// Insert 插入一条记录
func (RecordBestAverageImpl) Insert(record *models.RecordBestAverage) error {
	// 校验参数
	err := RecordBestAverage.check(record)
	if err != nil {
		return err
	}

	err = database.GetMySQL().Create(record).Error
	if err != nil {
		return err
	}

	// 往消息队列中发送消息
	RecordBestAverage.publishMessage(handlers.RankUpdate{
		Dimension: record.Dimension,
		Type:      record.Type,
	})

	return nil
}

// List 记录列表
func (RecordBestAverageImpl) List(recordReq *models.RecordBestAverageReq) (models.RecordBestAverageListResp, error) {
	var recordListResp models.RecordBestAverageListResp

	if recordReq.Username != "" || recordReq.Nickname != "" {
		userInfo, err := User.GetUserByUsernameOrNickname(recordReq.Username, recordReq.Nickname)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return recordListResp, errors.New("查询用户信息失败")
		}

		if userInfo.Id == "" {
			recordReq.UserIdStr = "-1"
		} else {
			recordReq.UserIdStr = userInfo.Id
		}
	}

	if recordReq.UserIdStr != "" {
		recordReq.UserId, _ = strconv.ParseInt(recordReq.UserIdStr, 10, 64)
	}

	if recordReq.IdStr != "" {
		recordReq.Id, _ = strconv.ParseInt(recordReq.IdStr, 10, 64)
	}

	if recordReq.OrderBy == "" {
		recordReq.OrderBy = "id"
	}

	db := database.GetMySQL().Table("record_best_average").Order(recordReq.OrderBy + " " + recordReq.Sorted)

	if recordReq.UserId != 0 {
		db.Where("user_id = ?", recordReq.UserId)
	}

	if recordReq.Dimension != 0 {
		db.Where("dimension = ?", recordReq.Dimension)
	}

	if recordReq.Type != 0 {
		db.Where("type = ?", recordReq.Type)
	}

	if len(recordReq.DurationRange) == 2 {
		if recordReq.DurationRange[0] != 0 {
			db.Where("record_duration >= ?", recordReq.DurationRange[0])
		}
		if recordReq.DurationRange[1] != 0 {
			db.Where("record_duration <= ?", recordReq.DurationRange[1])
		}
	}

	if len(recordReq.RankRange) == 2 {
		if recordReq.RankRange[0] != 0 {
			db.Where("ranked >= ?", recordReq.RankRange[0])
		}
		if recordReq.RankRange[01] != 0 {
			db.Where("ranked <= ?", recordReq.RankRange[1])
		}
	}

	if len(recordReq.BreakCountRange) == 2 {
		if recordReq.BreakCountRange[0] != 0 {
			db.Where("record_break_count >= ?", recordReq.BreakCountRange[0])
		}
		if recordReq.BreakCountRange[1] != 0 {
			db.Where("record_break_count <= ?", recordReq.BreakCountRange[1])
		}
	}

	if len(recordReq.DateRange) == 2 && !recordReq.DateRange[0].IsZero() && !recordReq.DateRange[1].IsZero() {
		db.Where("created_at >= ? AND created_at <= ?", recordReq.DateRange[0], recordReq.DateRange[1])
	}

	// 查询总数
	err := db.Count(&recordListResp.Total).Error
	if err != nil {
		return recordListResp, errors.New("记录总数查询失败")
	}

	// 分页
	if recordReq.Pagination.Page > 0 && recordReq.Pagination.PageSize > 0 {
		db.Scopes(utils.Paginate(&recordReq.Pagination))
	}

	if recordReq.NeedUserInfo {
		db.Preload("UserInfo")
	}

	// 查询列表
	err = db.Find(&recordListResp.Records).Error
	if err != nil {
		return recordListResp, errors.New("记录查询失败")
	}

	if recordReq.NeedRecordDetail {
		// 查询记录详情

		recordIds := make([]int64, 0)
		for _, record := range recordListResp.Records {
			// 逗号分隔的字符串转为数组
			recordIdArr := strings.Split(record.RecordIds, ",")
			for _, recordId := range recordIdArr {
				recordIdInt, _ := strconv.ParseInt(recordId, 10, 64)
				recordIds = append(recordIds, recordIdInt)
			}
		}

		recordList, err := Record.GetRecordByIds(recordIds)
		if err != nil {
			return recordListResp, errors.New("查询记录详情失败")
		}

		recordMap := make(map[string][]models.RecordResp)

		for _, record := range recordList.Records {
			key := record.UserId + "-" + strconv.Itoa(record.Dimension)
			recordMap[key] = append(recordMap[key], record)
		}

		for i, record := range recordListResp.Records {
			key := record.UserId + "-" + strconv.Itoa(record.Dimension)

			recordSet := mapset.NewSet()

			// 逗号分隔的字符串转为数组
			recordIdArr := strings.Split(record.RecordIds, ",")

			for _, recordId := range recordIdArr {
				recordSet.Add(recordId)
			}

			for _, record := range recordMap[key] {
				if recordSet.Contains(record.Id) {
					recordListResp.Records[i].RecordDetail.Records = append(recordListResp.Records[i].RecordDetail.Records, record)
				}
			}
			recordListResp.Records[i].RecordDetail.Total = int64(len(recordListResp.Records[i].RecordDetail.Records))
		}
	}

	return recordListResp, nil
}

// Update 更新记录
func (RecordBestAverageImpl) Update(record *models.RecordBestAverage) error {
	db := database.GetMySQL().Table("record_best_average").Where("user_id = ? AND dimension = ? AND type = ?", record.UserId, record.Dimension, record.Type)

	err := db.Updates(record).Error
	if err != nil {
		return err
	}

	// 往消息队列中发送消息
	RecordBestAverage.publishMessage(handlers.RankUpdate{
		Dimension: record.Dimension,
		Type:      record.Type,
	})

	return nil
}
