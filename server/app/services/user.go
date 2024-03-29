package services

import (
	"errors"
	"mime/multipart"
	"puzzle/app/models"
	"puzzle/database"
	"puzzle/utils"
	jwt "puzzle/utils/jwt"
	"strconv"
)

type UserService interface {
	Register(u *models.UserRegisterReq) error
	Login(u *models.UserLoginReq) (models.UserLoginResp, error)
	List(u *models.UserReq) (models.UserListResp, error)
	GetUserById(userId int64) (models.UserResp, error)
	GetUserByIdIds(userIds []int64) (models.UserListResp, error)
	GetUserByUsernameOrNickname(username string, nickname string)
	Update(u *models.User)
	UpdateAvatar(u *models.User, file *multipart.FileHeader)
}

type UserImpl struct{}

// Register 用户注册
func (UserImpl) Register(u *models.UserRegisterReq) error {

	// 判断用户名是否存在
	var tempUser models.User
	err := database.GetMySQL().Table("user").Where("username = ? AND status = ?", u.Username, 1).First(&tempUser).Error
	if err == nil {
		return errors.New("用户名已存在")
	}

	// 判断昵称是否存在
	_ = database.GetMySQL().Table("user").Where("nickname = ? AND status = ?", u.Nickname, 1).First(&tempUser).Error
	if tempUser.Id != 0 {
		return errors.New("昵称已存在")
	}

	snowflake := utils.Snowflake{}

	user := models.User{
		Id:       snowflake.NextVal(),
		Username: u.Username,
		Password: utils.MD5(u.Password),
		Nickname: u.Nickname,
		Status:   1,
	}

	return database.GetMySQL().Create(&user).Error
}

// Login 用户登录
func (UserImpl) Login(u *models.UserLoginReq) (models.UserLoginResp, error) {
	var loginResp models.UserLoginResp

	// 根据用户名获取用户信息
	var userInfo models.User
	err := database.GetMySQL().Table("user").Where("username = ?", u.Username).First(&userInfo).Error

	if err != nil {
		return loginResp, errors.New("用户不存在")
	}

	if userInfo.Password != utils.MD5(u.Password) {
		return loginResp, errors.New("密码错误")
	}

	if userInfo.Status == 2 {
		return loginResp, errors.New("用户已冻结")
	}

	if userInfo.Status == 3 {
		return loginResp, errors.New("用户已注销")
	}

	token, err := jwt.GenerateToken(userInfo.Id, userInfo.Username)
	if err != nil {
		return loginResp, errors.New("生成token失败")
	}

	// 返回用户信息
	loginResp.Token = token
	loginResp.User = models.UserResp{
		Id:        strconv.FormatInt(userInfo.Id, 10),
		Username:  userInfo.Username,
		Nickname:  userInfo.Nickname,
		Avatar:    userInfo.Avatar,
		Email:     userInfo.Email,
		Phone:     userInfo.Phone,
		Status:    userInfo.Status,
		CreatedAt: userInfo.CreatedAt,
		UpdatedAt: userInfo.UpdatedAt,
	}

	return loginResp, nil
}

// List 获取用户列表
func (UserImpl) List(u *models.UserReq) (models.UserListResp, error) {
	var userResp models.UserListResp

	if u.IdStr != "" {
		u.Id, _ = strconv.ParseInt(u.IdStr, 10, 64)
	}

	if u.OrderBy == "" {
		u.OrderBy = "id"
	}

	db := database.GetMySQL().Table("user").Order(u.OrderBy + " " + u.Sorted)

	if u.Id != 0 {
		db.Where("id = ?", u.Id)
	}

	if u.Username != "" {
		db.Where("username Like ?", "%"+u.Username+"%")
	}

	if u.Nickname != "" {
		db.Where("nickname Like ?", "%"+u.Nickname+"%")
	}

	if u.AccoladeId != 0 {
		db.Where("accolade_id = ?", u.AccoladeId)
	}

	if u.Email != "" {
		db.Where("email = ?", u.Email)
	}

	if u.Phone != "" {
		db.Where("phone = ?", u.Phone)
	}

	if u.Status != 0 {
		db.Where("status = ?", u.Status)
	}

	if len(u.DateRange) == 2 && !u.DateRange[0].IsZero() && !u.DateRange[1].IsZero() {
		db.Where("created_at >= ? and created_at <= ?", u.DateRange[0], u.DateRange[1])
	}

	if len(u.Ids) > 0 {
		db.Where("id in (?)", u.Ids)
	}

	// 查询总数
	err := db.Count(&userResp.Total).Error
	if err != nil {
		return userResp, errors.New("查询失败")
	}

	// 分页
	if u.Pagination.Page > 0 && u.Pagination.PageSize > 0 {
		db.Scopes(utils.Paginate(&u.Pagination))
	}

	// 查询列表
	err = db.Find(&userResp.Records).Error
	if err != nil {
		return userResp, errors.New("查询失败")
	}

	return userResp, nil
}

// GetAllUserId 获取所有用户id
func (UserImpl) GetAllUserId() ([]int64, error) {
	var userIds []int64
	err := database.GetMySQL().Table("user").Pluck("id", &userIds).Error
	if err != nil {
		return userIds, errors.New("查询失败")
	}

	return userIds, nil
}

// GetUserById 获取用户信息
func (UserImpl) GetUserById(userId int64) (models.UserResp, error) {
	var user models.UserResp
	err := database.GetMySQL().Table("user").Where("id = ? AND status = ?", userId, 1).First(&user).Error
	if err != nil {
		return user, errors.New("用户不存在")
	}

	return user, nil
}

// GetUserByIds 获取用户信息
func (UserImpl) GetUserByIds(userIds []int64) (models.UserListResp, error) {
	var userInfo models.UserListResp

	err := database.GetMySQL().Table("user").Where("id in ?", userIds).Find(&userInfo.Records).Error
	if err != nil {
		return userInfo, err
	}

	return userInfo, nil
}

// GetUserByUsernameOrNickname 根据用户名或昵称获取用户信息
func (UserImpl) GetUserByUsernameOrNickname(username string, nickname string) (models.UserResp, error) {
	var user models.UserResp

	db := database.GetMySQL().Table("user")

	if username != "" {
		db.Where("username Like ?", "%"+username+"%")
	}

	if nickname != "" {
		db.Where("nickname Like ?", "%"+nickname+"%")
	}

	err := db.First(&user).Error
	return user, err
}

// Update 更新用户
func (UserImpl) Update(u *models.User) error {

	if u.Password != "" {
		u.Password = utils.MD5(u.Password)
	}

	// 删除用户
	if u.Status == 3 {
		var user models.User
		err := database.GetMySQL().Table("user").Where("id = ?", u.Id).First(&user).Error
		if err != nil {
			return errors.New("用户不存在")
		}

		u.Username = user.Username + "_del"
		u.Nickname = user.Nickname + "_del"
	}

	err := database.GetMySQL().Table("user").Updates(u).Error
	if err != nil {
		return errors.New("更新失败")
	}

	return nil
}

// UpdateAvatar 更新用户头像
func (UserImpl) UpdateAvatar(u *models.User, file *multipart.FileHeader) error {
	// 上传头像
	filePath, err := Cos.UploadAvatar(file)
	if err != nil {
		return err
	}

	var user models.User
	err = database.GetMySQL().Table("user").Where("id = ? AND status = ?", u.Id, 1).First(&user).Error
	if err != nil {
		return errors.New("用户不存在")
	}

	// 删除旧头像
	if user.Avatar != "" {
		err = Cos.DeleteAvatar(user.Avatar)
		if err != nil {
			return err
		}
	}

	u.Avatar = filePath

	// 更新头像
	err = database.GetMySQL().Table("user").Updates(u).Error
	if err != nil {
		return errors.New("更新用户头像失败")
	}

	return nil
}
