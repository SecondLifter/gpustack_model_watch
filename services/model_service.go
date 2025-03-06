package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type ModelService struct {
	baseURL  string
	cookies  []*http.Cookie
	client   *http.Client
	username string
	password string
}

type ModelInstance struct {
	ID           int    `json:"id"`
	ModelID      int    `json:"model_id"`
	ModelName    string `json:"model_name"`
	State        string `json:"state"`
	StateMessage string `json:"state_message"`
}

type ModelInstanceResponse struct {
	Items      []ModelInstance `json:"items"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 添加重试相关的常量
const (
	maxRetries = 3
	retryDelay = 5 * time.Second
)

func NewModelService(baseURL, username, password string) *ModelService {
	return &ModelService{
		baseURL:  baseURL,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Login 执行登录并保存cookie
func (s *ModelService) Login() error {
	formData := fmt.Sprintf("username=%s&password=%s", s.username, s.password)

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/auth/login", s.baseURL),
		bytes.NewBufferString(formData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("登录请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("登录失败，状态码: %d", resp.StatusCode)
	}

	// 保存cookies
	s.cookies = resp.Cookies()
	log.Printf("登录成功，获取到 %d 个 cookies", len(s.cookies))
	return nil
}

// addCookies 将保存的cookies添加到请求中
func (s *ModelService) addCookies(req *http.Request) {
	for _, cookie := range s.cookies {
		req.AddCookie(cookie)
	}
}

// WatchErrorModels 监控错误状态的模型并删除
func (s *ModelService) WatchErrorModels() {
	for {
		// 如果没有cookies，尝试登录
		if len(s.cookies) == 0 {
			if err := s.Login(); err != nil {
				log.Printf("登录失败: %v", err)
				time.Sleep(30 * time.Second)
				continue
			}
		}

		// 获取所有模型列表
		models, err := s.getModels()
		if err != nil {
			log.Printf("获取模型列表失败: %v", err)
			// 如果是认证错误，清除cookies并重新登录
			if err.Error() == "API返回错误状态码: 401" {
				s.cookies = nil
			}
			time.Sleep(30 * time.Second)
			continue
		}

		// 检查每个模型的实例状态
		for _, model := range models {
			instances, err := s.getModelInstances(model.ID)
			if err != nil {
				log.Printf("获取模型 %d 实例状态失败: %v", model.ID, err)
				continue
			}

			// 检查实例状态
			for _, instance := range instances {
				log.Printf("instance: %+v", instance)
				if instance.State == "error" {
					log.Printf("发现错误状态的模型: ID=%d, Name=%s, Message=%s",
						instance.ModelID, instance.ModelName, instance.StateMessage)

					// 删除错误状态的模型
					if err := s.deleteModel(instance.ID); err != nil {
						log.Printf("删除模型 %d 失败: %v", instance.ModelID, err)
					} else {
						log.Printf("成功删除错误状态的模型: ID=%d, Name=%s",
							instance.ModelID, instance.ModelName)
					}
				}
			}
		}

		// 等待30秒后继续下一轮检查
		time.Sleep(30 * time.Second)
	}
}

// 添加重试登录的方法
func (s *ModelService) retryWithLogin(operation func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// 非首次尝试，先等待一段时间
			time.Sleep(retryDelay)
		}

		// 如果没有cookies或上次请求返回401，先尝试登录
		if len(s.cookies) == 0 {
			if err = s.Login(); err != nil {
				log.Printf("第 %d 次登录尝试失败: %v", i+1, err)
				continue
			}
		}

		// 执行操作
		err = operation()
		if err == nil {
			return nil
		}

		// 检查是否是认证错误
		if err.Error() == "API返回错误状态码: 401" {
			log.Printf("认证失败，清除 cookies 并准备重试")
			s.cookies = nil
			continue
		}

		// 如果是其他错误，直接返回
		return err
	}
	return fmt.Errorf("重试 %d 次后仍然失败: %v", maxRetries, err)
}

// 修改 getModels 方法
func (s *ModelService) getModels() ([]struct {
	ID int `json:"id"`
}, error) {
	var result []struct {
		ID int `json:"id"`
	}
	err := s.retryWithLogin(func() error {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/models", s.baseURL), nil)
		if err != nil {
			return err
		}

		s.addCookies(req)

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("API返回错误状态码: 401")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
		}

		var response struct {
			Items []struct {
				ID int `json:"id"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return err
		}

		result = response.Items
		return nil
	})

	return result, err
}

// 修改 getModelInstances 方法
func (s *ModelService) getModelInstances(modelID int) ([]ModelInstance, error) {
	var result []ModelInstance
	err := s.retryWithLogin(func() error {
		req, err := http.NewRequest("GET",
			fmt.Sprintf("%s/v1/models/%d/instances", s.baseURL, modelID), nil)
		if err != nil {
			return err
		}

		s.addCookies(req)

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("API返回错误状态码: 401")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API返回错误状态码: %d", resp.StatusCode)
		}

		var response ModelInstanceResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return err
		}

		result = response.Items
		return nil
	})

	return result, err
}

// 修改 deleteModel 方法
func (s *ModelService) deleteModel(ID int) error {
	return s.retryWithLogin(func() error {
		req, err := http.NewRequest("DELETE",
			fmt.Sprintf("%s/v1/model-instances/%d", s.baseURL, ID), nil)
		if err != nil {
			return err
		}

		s.addCookies(req)

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("API返回错误状态码: 401")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("删除模型失败，API返回状态码: %d", resp.StatusCode)
		}

		return nil
	})
}
