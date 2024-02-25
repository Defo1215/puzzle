package record

import (
	"errors"
	"fmt"
	"puzzle/app/models"

	commonService "puzzle/app/services"
	recordBestAverageSerivce "puzzle/app/services/record-best-average"
	recordBestSingleSerivce "puzzle/app/services/record-best-single"
	recordBestStepSerivce "puzzle/app/services/record-best-step"
	scrambledUserStatusService "puzzle/app/services/scrambled-user-status"
	"puzzle/database"
	"puzzle/utils"
	"sort"
	"strconv"
	"strings"
)

// check 检查参数
func check(record models.Record) error {
	if record.UserId == 0 {
		return errors.New("用户ID不能为空")
	}

	if record.Dimension == 0 {
		return errors.New("阶数不能为空")
	}

	if record.Type == 0 {
		return errors.New("类型不能为空")
	}

	if record.Duration == 0 {
		return errors.New("时长不能为空")
	}

	if record.Step == 0 {
		return errors.New("步数不能为空")
	}

	if record.Scramble == "" {
		return errors.New("打乱公式不能为空")
	}

	if record.Solution == "" {
		return errors.New("解法不能为空")
	}

	if record.Idx == 0 {
		return errors.New("打乱随机数不能为空")
	}

	return nil
}

// Insert 新增记录
func Insert(record models.Record) error {
	// 检查参数
	err := check(record)
	if err != nil {
		return err
	}

	snowflake := utils.Snowflake{}

	record.Id = snowflake.NextVal() // 生成ID
	record.Status = 1               // 默认状态为1

	// 插入记录
	err = database.GetMySQL().Create(&record).Error
	if err != nil {
		return errors.New("新增失败")
	}

	// 若记录为非练习记录, 则需要更新用户的记录
	if record.Type != 1 {
		// 更新用户的完成状态
		scrambledUserStatus, err := scrambledUserStatusService.List(models.ScrambledUserStatusReq{
			UserId:    record.UserId,
			Dimension: record.Dimension,
			Pagination: utils.Pagination{
				PageSize: 1,
				Page:     1,
			}})

		if err != nil {
			return errors.New("获取用户打乱状态失败")
		}

		id, _ := strconv.ParseInt(scrambledUserStatus.Records[0].Id, 10, 64)

		err = scrambledUserStatusService.Update(models.ScrambledUserStatus{
			Id:     id,
			Status: 2,
		})

		if err != nil {
			return err
		}

		// 更新用户最佳单次记录
		err = updateRecordBestSingle(record)
		if err != nil {
			return err
		}

		// 更新用户最佳5次平均记录
		err = updateRecordBestAverage5(record)
		if err != nil {
			return err
		}

		// 更新用户最佳12次平均记录
		err = updateRecordBestAverage12(record)
		if err != nil {
			return err
		}

		// 更新用户最佳步数记录
		err = updateRecordBestStep(record)
		if err != nil {
			return err
		}
	}

	return nil
}

// List 记录列表
func List(recordReq models.RecordReq) (models.RecordListResp, error) {
	var recordListResp models.RecordListResp

	if recordReq.Username != "" || recordReq.Nickname != "" {
		userInfo, err := commonService.GetUserInfoByUsernameOrNickname(recordReq.Username, recordReq.Nickname)
		if err != nil {
			return recordListResp, errors.New("查询用户信息失败")
		}

		if userInfo.Id == "" {
			return recordListResp, errors.New("用户不存在")
		}

		recordReq.UserIdStr = userInfo.Id
	}

	recordReq.Id, _ = strconv.ParseInt(recordReq.IdStr, 10, 64)
	recordReq.UserId, _ = strconv.ParseInt(recordReq.UserIdStr, 10, 64)
	recordReq.Idx, _ = strconv.ParseInt(recordReq.IdxStr, 10, 64)

	if recordReq.OrderBy == "" {
		recordReq.OrderBy = "id"
	}

	db := database.GetMySQL().Table("record").Order(recordReq.OrderBy + " " + recordReq.Sorted)

	if recordReq.Id != 0 {
		db = db.Where("id = ?", recordReq.Id)
	}

	if recordReq.UserId != 0 {
		db = db.Where("user_id = ?", recordReq.UserId)
	}

	if recordReq.Dimension != 0 {
		db = db.Where("dimension = ?", recordReq.Dimension)
	}

	if recordReq.Type != 0 {
		db = db.Where("type = ?", recordReq.Type)
	}

	if len(recordReq.DurationRange) == 2 {
		if recordReq.DurationRange[0] != 0 {
			db = db.Where("duration >= ?", recordReq.DurationRange[0])
		}
		if recordReq.DurationRange[1] != 0 {
			db = db.Where("duration <= ?", recordReq.DurationRange[1])
		}
	}

	if len(recordReq.StepRange) == 2 {
		if recordReq.StepRange[0] != 0 {
			db = db.Where("step >= ?", recordReq.StepRange[0])
		}
		if recordReq.StepRange[1] != 0 {
			db = db.Where("step <= ?", recordReq.StepRange[1])
		}
	}

	if len(recordReq.DateRange) == 2 && !recordReq.DateRange[0].IsZero() && !recordReq.DateRange[1].IsZero() {
		db = db.Where("created_at >= ? AND created_at <= ?", recordReq.DateRange[0], recordReq.DateRange[1])
	}

	if recordReq.Idx != 0 {
		db = db.Where("idx = ?", recordReq.Idx)
	}

	if len(recordReq.Ids) > 0 {
		db = db.Where("id in (?)", recordReq.Ids)
	}

	if recordReq.Status != 0 {
		db = db.Where("status = ?", recordReq.Status)
	}

	// 查询总数
	err := db.Count(&recordListResp.Total).Error
	if err != nil {
		return recordListResp, errors.New("查询失败")
	}

	// 分页
	if recordReq.Pagination.Page > 0 && recordReq.Pagination.PageSize > 0 {
		db = db.Scopes(utils.Paginate(&recordReq.Pagination))
	}

	// 查询记录
	err = db.Find(&recordListResp.Records).Error
	if err != nil {
		return recordListResp, errors.New("查询失败")
	}

	// 查询用户信息
	if recordReq.NeedUserInfo {
		userIds := make([]int64, 0)
		for _, record := range recordListResp.Records {
			userId, _ := strconv.ParseInt(record.UserId, 10, 64)
			userIds = append(userIds, userId)
		}

		userList, err := commonService.GetUserInfo(userIds)
		if err != nil {
			return recordListResp, errors.New("查询用户信息失败")
		}

		userMap := make(map[string]models.UserResp)
		for _, user := range userList.Records {
			userMap[user.Id] = user
		}

		for i, record := range recordListResp.Records {
			recordListResp.Records[i].UserInfo = userMap[record.UserId]
		}
	}

	return recordListResp, nil
}

// Update 更新记录
func Update(record models.Record) error {
	err := database.GetMySQL().Model(&record).Updates(&record).Error
	if err != nil {
		return errors.New("更新失败")
	}

	return nil
}

// updateRecordBestSingle 更新最佳单次记录
func updateRecordBestSingle(record models.Record) error {

	// 获取最佳单次记录
	recordBestSingle, err := recordBestSingleSerivce.List(models.RecordBestSingleReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,

		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 1,
		},
	})
	if err != nil {
		return errors.New("获取最佳单次记录失败")
	}

	// 若无最佳单次记录, 则直接插入
	if recordBestSingle.Total == 0 {
		snowflake := utils.Snowflake{}

		err = recordBestSingleSerivce.Insert(models.RecordBestSingle{
			Id:               snowflake.NextVal(),
			UserId:           record.UserId,
			Dimension:        record.Dimension,
			RecordId:         record.Id,
			RecordDuration:   record.Duration,
			RecordStep:       record.Step,
			RecordBreakCount: 1,
		})

		if err != nil {
			return errors.New("新增最佳单次记录失败")
		}
	} else {
		// 若有最佳单次记录, 则比较并更新
		if record.Duration < recordBestSingle.Records[0].RecordDuration {
			id, _ := strconv.ParseInt(recordBestSingle.Records[0].Id, 10, 64)

			err = recordBestSingleSerivce.Update(models.RecordBestSingle{
				Id:               id,
				RecordId:         record.Id,
				RecordDuration:   record.Duration,
				RecordStep:       record.Step,
				RecordBreakCount: recordBestSingle.Records[0].RecordBreakCount + 1,
			})

			if err != nil {
				return errors.New("更新最佳单次记录失败")
			}
		}
	}

	return nil
}

// updateRecordBestAverag5e 更新最佳五次平均记录
func updateRecordBestAverage5(record models.Record) error {

	// 获取用户最近5条记录
	last5Records, err := List(models.RecordReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,
		Type:      record.Type,
		Status:    1,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 5,
		},
		Sorted: "desc",
	})

	if err != nil {
		return errors.New("获取最近5条记录失败")
	}

	// 若记录数小于5, 则无法计算平均记录
	if last5Records.Total < 5 {
		return nil
	}

	var totalDuration int

	// 将记录的持续时间存储到一个切片中
	durations := make([]int, 0, len(last5Records.Records))
	for _, v := range last5Records.Records {
		durations = append(durations, int(v.Duration))
	}

	// 对切片进行排序
	sort.Ints(durations)

	// 去掉最大和最小值
	for i := 1; i < len(durations)-1; i++ {
		totalDuration += durations[i]
	}

	// 计算平均值
	averageDuration := totalDuration / (len(durations) - 2)

	// 获取最佳平均记录
	recordBestAverage, err := recordBestAverageSerivce.List(models.RecordBestAverageReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,
		Type:      5,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 1,
		},
	})

	if err != nil {
		return errors.New("获取最佳平均记录失败")
	}

	// 整合最近5条记录id
	var recordIds []string
	for _, v := range last5Records.Records {
		recordIds = append(recordIds, v.Id)
	}

	recordIdsStr := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(recordIds)), ","), "[]")

	// 若无最佳平均记录, 则直接插入
	if recordBestAverage.Total == 0 {
		snowflake := utils.Snowflake{}

		err = recordBestAverageSerivce.Insert(models.RecordBestAverage{
			Id:                    snowflake.NextVal(),
			UserId:                record.UserId,
			Dimension:             record.Dimension,
			Type:                  5,
			RecordIds:             recordIdsStr,
			RecordAverageDuration: averageDuration,
			RecordBreakCount:      1,
		})

		if err != nil {
			return errors.New("新增最佳平均记录失败")
		}
	} else {

		id, _ := strconv.ParseInt(recordBestAverage.Records[0].Id, 10, 64)

		// 若有最佳平均记录, 则比较并更新
		if averageDuration < recordBestAverage.Records[0].RecordAverageDuration {
			err = recordBestAverageSerivce.Update(models.RecordBestAverage{
				Id:                    id,
				RecordIds:             recordIdsStr,
				RecordAverageDuration: averageDuration,
				RecordBreakCount:      recordBestAverage.Records[0].RecordBreakCount + 1,
			})

			if err != nil {
				return errors.New("更新最佳平均记录失败")
			}
		}
	}

	return nil
}

// updateRecordBestAverage12 更新最佳12次平均记录
func updateRecordBestAverage12(record models.Record) error {
	// 获取用户最近12条记录
	last12Records, err := List(models.RecordReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,
		Type:      record.Type,
		Status:    1,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 12,
		},
		Sorted: "desc",
	})

	if err != nil {
		return errors.New("获取最近12条记录失败")
	}

	// 若记录数小于12, 则无法计算平均记录
	if last12Records.Total < 12 {
		return nil
	}

	var totalDuration int

	// 将记录的持续时间存储到一个切片中
	durations := make([]int, 0, len(last12Records.Records))
	for _, v := range last12Records.Records {
		durations = append(durations, int(v.Duration))
	}

	// 对切片进行排序
	sort.Ints(durations)

	// 去掉最大和最小值
	for i := 1; i < len(durations)-1; i++ {
		totalDuration += durations[i]
	}

	// 计算平均值
	averageDuration := totalDuration / (len(durations) - 2)

	// 获取最佳平均记录
	recordBestAverage, err := recordBestAverageSerivce.List(models.RecordBestAverageReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,
		Type:      12,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 1,
		},
	})

	if err != nil {
		return errors.New("获取最佳平均记录失败")
	}

	// 整合最近12条记录id
	var recordIds []string
	for _, v := range last12Records.Records {
		recordIds = append(recordIds, v.Id)
	}

	recordIdsStr := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(recordIds)), ","), "[]")

	// 若无最佳平均记录, 则直接插入
	if recordBestAverage.Total == 0 {
		snowflake := utils.Snowflake{}

		err = recordBestAverageSerivce.Insert(models.RecordBestAverage{
			Id:                    snowflake.NextVal(),
			UserId:                record.UserId,
			Dimension:             record.Dimension,
			Type:                  12,
			RecordIds:             recordIdsStr,
			RecordAverageDuration: averageDuration,
			RecordBreakCount:      1,
		})

		if err != nil {
			return errors.New("新增最佳平均记录失败")
		}
	} else {

		id, _ := strconv.ParseInt(recordBestAverage.Records[0].Id, 10, 64)

		// 若有最佳平均记录, 则比较并更新
		if averageDuration < recordBestAverage.Records[0].RecordAverageDuration {
			err = recordBestAverageSerivce.Update(models.RecordBestAverage{
				Id:                    id,
				RecordIds:             recordIdsStr,
				RecordAverageDuration: averageDuration,
				RecordBreakCount:      recordBestAverage.Records[0].RecordBreakCount + 1,
			})

			if err != nil {
				return errors.New("更新最佳平均记录失败")
			}
		}
	}
	return nil
}

// updateRecordBestStep 更新最佳步数记录
func updateRecordBestStep(record models.Record) error {

	// 获取用户最佳步数记录
	recordBestStep, err := recordBestStepSerivce.List(models.RecordBestStepReq{
		UserId:    record.UserId,
		Dimension: record.Dimension,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 1,
		},
	})

	if err != nil {
		return errors.New("获取最佳步数记录失败")
	}

	// 若无最佳步数记录, 则直接插入
	if recordBestStep.Total == 0 {

		snowflake := utils.Snowflake{}

		err = recordBestStepSerivce.Insert(models.RecordBestStep{
			Id:               snowflake.NextVal(),
			UserId:           record.UserId,
			Dimension:        record.Dimension,
			RecordId:         record.Id,
			RecordStep:       record.Step,
			RecordBreakCount: 1,
		})

		if err != nil {
			return errors.New("新增最佳步数记录失败")
		}

	} else {

		id, _ := strconv.ParseInt(recordBestStep.Records[0].Id, 10, 64)

		// 若有最佳步数记录, 则比较并更新
		if record.Step < recordBestStep.Records[0].RecordStep {
			err = recordBestStepSerivce.Update(models.RecordBestStep{
				Id:               id,
				RecordId:         record.Id,
				RecordStep:       record.Step,
				RecordBreakCount: recordBestStep.Records[0].RecordBreakCount + 1,
			})

			if err != nil {
				return errors.New("更新最佳步数记录失败")
			}
		}
	}
	return nil
}
