package record

import (
	"errors"
	"fmt"
	"puzzle/app/models"
	recordBestAverageSerivce "puzzle/app/services/record-best-average"
	recordBestSingleSerivce "puzzle/app/services/record-best-single"
	recordBestStepSerivce "puzzle/app/services/record-best-step"
	"puzzle/database"
	"puzzle/utils"
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

	// if record.Type == 0 {
	// 	return errors.New("类型不能为空")
	// }

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
	record.Status = 0               // 默认状态为0

	// 插入记录
	err = database.GetMySQL().Create(&record).Error
	if err != nil {
		return errors.New("新增失败")
	}

	// 若记录为非练习记录, 则需要更新用户的记录
	if record.Type != 0 {
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
	db := database.GetMySQL().Table("record").Order("created_at "+recordReq.Sorted).Where("type = ?", recordReq.Type)

	if recordReq.Id != 0 {
		db = db.Where("id = ?", recordReq.Id)
	}

	if recordReq.UserId != 0 {
		db = db.Where("user_id = ?", recordReq.UserId)
	}

	if recordReq.Dimension != 0 {
		db = db.Where("dimension = ?", recordReq.Dimension)
	}

	if len(recordReq.DurationRange) == 2 {
		db = db.Where("duration >= ? AND duration <= ?", recordReq.DurationRange[0], recordReq.DurationRange[1])
	}

	if len(recordReq.StepRange) == 2 {
		db = db.Where("step >= ? AND step <= ?", recordReq.StepRange[0], recordReq.StepRange[1])
	}

	if recordReq.Status != 0 {
		db = db.Where("status = ?", recordReq.Status)
	}

	if len(recordReq.DateRange) == 2 {
		db = db.Where("created_at >= ? AND created_at <= ?", recordReq.DateRange[0], recordReq.DateRange[1])
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
		err = recordBestSingleSerivce.Insert(models.RecordBestSingle{
			UserId:         record.UserId,
			Dimension:      record.Dimension,
			RecordId:       record.Id,
			RecordDuration: record.Duration,
			RecordStep:     record.Step,
		})

		if err != nil {
			return errors.New("新增最佳单次记录失败")
		}
	} else {
		// 若有最佳单次记录, 则比较并更新
		if record.Duration < recordBestSingle.Records[0].RecordDuration {
			err = recordBestSingleSerivce.Update(models.RecordBestSingle{
				UserId:         record.UserId,
				Dimension:      record.Dimension,
				RecordId:       record.Id,
				RecordDuration: record.Duration,
				RecordStep:     record.Step,
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
		Status:    0,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 5,
		},
	})

	if err != nil {
		return errors.New("获取最近5条记录失败")
	}

	// 若记录数小于5, 则无法计算平均记录
	if last5Records.Total < 5 {
		return nil
	}

	// 计算平均记录
	var totalDuration int
	for _, v := range last5Records.Records {
		totalDuration += int(v.Duration)
	}

	averageDuration := totalDuration / 5

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
	var recordIds []int64
	for _, v := range last5Records.Records {
		recordIds = append(recordIds, v.Id)
	}

	recordIdsStr := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(recordIds)), ","), "[]")

	// 若无最佳平均记录, 则直接插入
	if recordBestAverage.Total == 0 {
		err = recordBestAverageSerivce.Insert(models.RecordBestAverage{
			UserId:                record.UserId,
			Dimension:             record.Dimension,
			Type:                  5,
			RecordIds:             recordIdsStr,
			RecordAverageDuration: averageDuration,
		})

		if err != nil {
			return errors.New("新增最佳平均记录失败")
		}
	} else {
		// 若有最佳平均记录, 则比较并更新
		if averageDuration < recordBestAverage.Records[0].RecordAverageDuration {
			err = recordBestAverageSerivce.Update(models.RecordBestAverage{
				UserId:                record.UserId,
				Dimension:             record.Dimension,
				Type:                  5,
				RecordIds:             recordIdsStr,
				RecordAverageDuration: averageDuration,
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
		Status:    0,
		Pagination: utils.Pagination{
			Page:     1,
			PageSize: 12,
		},
	})

	if err != nil {
		return errors.New("获取最近12条记录失败")
	}

	// 若记录数小于12, 则无法计算平均记录
	if last12Records.Total < 12 {
		return nil
	}

	// 计算平均记录
	var totalDuration int
	for _, v := range last12Records.Records {
		totalDuration += int(v.Duration)
	}

	averageDuration := totalDuration / 12

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

	// 整合最近5条记录id
	var recordIds []int64
	for _, v := range last12Records.Records {
		recordIds = append(recordIds, v.Id)
	}

	recordIdsStr := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(recordIds)), ","), "[]")

	// 若无最佳平均记录, 则直接插入
	if recordBestAverage.Total == 0 {
		err = recordBestAverageSerivce.Insert(models.RecordBestAverage{
			UserId:                record.UserId,
			Dimension:             record.Dimension,
			Type:                  12,
			RecordIds:             recordIdsStr,
			RecordAverageDuration: averageDuration,
		})

		if err != nil {
			return errors.New("新增最佳平均记录失败")
		}
	} else {
		// 若有最佳平均记录, 则比较并更新
		if averageDuration < recordBestAverage.Records[0].RecordAverageDuration {
			err = recordBestAverageSerivce.Update(models.RecordBestAverage{
				UserId:                record.UserId,
				Dimension:             record.Dimension,
				Type:                  12,
				RecordIds:             recordIdsStr,
				RecordAverageDuration: averageDuration,
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
		err = recordBestStepSerivce.Insert(models.RecordBestStep{
			UserId:     record.UserId,
			Dimension:  record.Dimension,
			RecordId:   record.Id,
			RecordStep: record.Step,
		})

		if err != nil {
			return errors.New("新增最佳步数记录失败")
		}

	} else {
		// 若有最佳步数记录, 则比较并更新
		if record.Step < recordBestStep.Records[0].RecordStep {
			err = recordBestStepSerivce.Update(models.RecordBestStep{
				UserId:     record.UserId,
				Dimension:  record.Dimension,
				RecordId:   record.Id,
				RecordStep: record.Step,
			})

			if err != nil {
				return errors.New("更新最佳步数记录失败")
			}
		}
	}
	return nil
}
