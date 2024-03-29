package services

import (
	"errors"
	"puzzle/app/models"
	"puzzle/database"
	"puzzle/utils"
)

type ScrambledUserStatusService interface {
	check(scrambledUserStatus *models.ScrambledUserStatus) error
	Insert(scrambledUserStatus *models.ScrambledUserStatus) error
	List(scrambledUserStatusReq *models.ScrambledUserStatusReq) (models.ScrambledUserStatusListResp, error)
	Update(scrambledUserStatus *models.ScrambledUserStatus) error
}

type ScrambledUserStatusImpl struct{}

func (ScrambledUserStatusImpl) check(scrambledUserStatus *models.ScrambledUserStatus) error {
	if scrambledUserStatus.UserId == 0 {
		return errors.New("用户ID不能为空")
	}

	if scrambledUserStatus.Dimension == 0 {
		return errors.New("阶数不能为空")
	}

	if scrambledUserStatus.ScrambleId == 0 {
		return errors.New("打乱公式ID不能为空")
	}

	return nil
}

func (ScrambledUserStatusImpl) Insert(scrambledUserStatus *models.ScrambledUserStatus) error {
	if err := ScrambledUserStatus.check(scrambledUserStatus); err != nil {
		return err
	}

	snowflake := utils.Snowflake{}

	scrambledUserStatus.Id = snowflake.NextVal()
	scrambledUserStatus.Status = 1

	return database.GetMySQL().Create(scrambledUserStatus).Error
}

func (ScrambledUserStatusImpl) List(scrambledUserStatusReq *models.ScrambledUserStatusReq) (models.ScrambledUserStatusListResp, error) {
	var scrambledUserStatusListResp models.ScrambledUserStatusListResp
	db := database.GetMySQL().Table("scrambled_user_status")

	if scrambledUserStatusReq.UserId != 0 {
		db.Where("user_id = ?", scrambledUserStatusReq.UserId)
	}

	if scrambledUserStatusReq.Dimension != 0 {
		db.Where("dimension = ?", scrambledUserStatusReq.Dimension)
	}

	if scrambledUserStatusReq.ScrambleId != 0 {
		db.Where("scramble_id = ?", scrambledUserStatusReq.ScrambleId)
	}

	if scrambledUserStatusReq.Status != 0 {
		db.Where("status = ?", scrambledUserStatusReq.Status)
	}

	if len(scrambledUserStatusReq.DateRange) == 2 {
		db.Where("created_at BETWEEN ? AND ?", scrambledUserStatusReq.DateRange[0], scrambledUserStatusReq.DateRange[1])
	}

	if scrambledUserStatusReq.Sorted != "" {
		db.Order("created_at " + scrambledUserStatusReq.Sorted)
	}

	// 查询总数
	if err := db.Count(&scrambledUserStatusListResp.Total).Error; err != nil {
		return scrambledUserStatusListResp, errors.New("查询失败")
	}

	// 分页
	if scrambledUserStatusReq.Pagination.Page > 0 && scrambledUserStatusReq.Pagination.PageSize > 0 {
		db.Scopes(utils.Paginate(&scrambledUserStatusReq.Pagination))
	}

	// 查询记录
	if err := db.Find(&scrambledUserStatusListResp.Records).Error; err != nil {
		return scrambledUserStatusListResp, errors.New("查询失败")
	}

	return scrambledUserStatusListResp, nil
}

func (ScrambledUserStatusImpl) Update(scrambledUserStatus *models.ScrambledUserStatus) error {
	db := database.GetMySQL().Table("scrambled_user_status")

	err := db.Updates(scrambledUserStatus).Error
	if err != nil {
		return errors.New("更新失败")
	}

	return nil
}
