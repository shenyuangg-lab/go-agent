package scheduler

import (
	"time"

	"go-agent/pkg/client"
	"go-agent/pkg/services"

	"github.com/sirupsen/logrus"
)

// CustomTrigger 自定义触发器，基于Java的CustomScheduleTrigger实现
type CustomTrigger struct {
	monitorItem *services.CollectItem
	intervals   []*client.ItemCustomInterval
	logger      *logrus.Logger
}

// NewCustomTrigger 创建自定义触发器
func NewCustomTrigger(monitorItem *services.CollectItem, logger *logrus.Logger) *CustomTrigger {
	return &CustomTrigger{
		monitorItem: monitorItem,
		intervals:   monitorItem.Intervals,
		logger:      logger,
	}
}

// NextExecutionTime 计算下次执行时间，实现Java版本的nextExecutionTime逻辑
func (ct *CustomTrigger) NextExecutionTime(lastCompletionTime *time.Time) time.Time {
	// 上次完成时间，如果为空则使用当前时间
	var completionTime time.Time
	if lastCompletionTime == nil {
		completionTime = time.Now()
	} else {
		completionTime = *lastCompletionTime
	}

	// 获取当前时间
	now := time.Now()
	currentTime := now.Format("15:04:05") // HH:mm:ss格式
	dayOfWeek := int(now.Weekday())

	// 星期天为0，但Java中星期天为7，需要转换
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}

	// 如果intervals数组为空或长度为0，则直接使用updateIntervalSeconds
	if len(ct.intervals) == 0 {
		if ct.monitorItem.UpdateIntervalSeconds > 0 {
			nextTime := completionTime.Add(time.Duration(ct.monitorItem.UpdateIntervalSeconds) * time.Second)
			ct.logger.Debugf("intervals为空，使用updateIntervalSeconds间隔 %d 秒，下次执行时间: %v", 
				ct.monitorItem.UpdateIntervalSeconds, nextTime)
			return nextTime
		}
		// 如果updateIntervalSeconds也为0，返回零值
		ct.logger.Warnf("监控项 %s 既没有自定义间隔也没有updateIntervalSeconds配置", ct.monitorItem.ItemName)
		return time.Time{}
	}

	// 检查是否处于自定义间隔范围内
	for _, interval := range ct.intervals {
		if ct.isInCustomInterval(interval, dayOfWeek, currentTime) {
			if interval.IntervalSeconds > 0 {
				nextTime := completionTime.Add(time.Duration(interval.IntervalSeconds) * time.Second)
				ct.logger.Debugf("处于自定义间隔范围内，使用自定义间隔 %d 秒，下次执行时间: %v", 
					interval.IntervalSeconds, nextTime)
				return nextTime
			}
		}
	}

	// 如果不在自定义时间的区间内，则使用updateIntervalSeconds
	if ct.monitorItem.UpdateIntervalSeconds > 0 {
		nextTime := completionTime.Add(time.Duration(ct.monitorItem.UpdateIntervalSeconds) * time.Second)
		ct.logger.Debugf("不在自定义间隔范围内，使用updateIntervalSeconds间隔 %d 秒，下次执行时间: %v", 
			ct.monitorItem.UpdateIntervalSeconds, nextTime)
		return nextTime
	}

	// 如果没有配置任何间隔，返回零值（不再执行）
	ct.logger.Warnf("监控项 %s 没有配置执行间隔", ct.monitorItem.ItemName)
	return time.Time{}
}

// isInCustomInterval 检查当前时间是否在自定义间隔范围内
func (ct *CustomTrigger) isInCustomInterval(interval *client.ItemCustomInterval, dayOfWeek int, currentTime string) bool {
	// 检查星期是否匹配
	if interval.Week != dayOfWeek {
		return false
	}

	// 检查时间范围
	if interval.StartTime == "" || interval.EndTime == "" {
		ct.logger.Warnf("自定义间隔的开始时间或结束时间为空")
		return false
	}

	// 解析时间
	startTime, err := time.Parse("15:04:05", interval.StartTime)
	if err != nil {
		ct.logger.Errorf("解析开始时间失败 %s: %v", interval.StartTime, err)
		return false
	}

	endTime, err := time.Parse("15:04:05", interval.EndTime)
	if err != nil {
		ct.logger.Errorf("解析结束时间失败 %s: %v", interval.EndTime, err)
		return false
	}

	current, err := time.Parse("15:04:05", currentTime)
	if err != nil {
		ct.logger.Errorf("解析当前时间失败 %s: %v", currentTime, err)
		return false
	}

	// 检查当前时间是否在范围内 (startTime < current < endTime)
	return current.After(startTime) && current.Before(endTime)
}

// GetMonitorItem 获取监控项
func (ct *CustomTrigger) GetMonitorItem() *services.CollectItem {
	return ct.monitorItem
}

// HasCustomInterval 检查是否有自定义间隔配置
func (ct *CustomTrigger) HasCustomInterval() bool {
	return len(ct.intervals) > 0
}

// ShouldExecuteNow 检查当前时间是否应该执行
func (ct *CustomTrigger) ShouldExecuteNow() bool {
	now := time.Now()
	currentTime := now.Format("15:04:05")
	dayOfWeek := int(now.Weekday())

	if dayOfWeek == 0 {
		dayOfWeek = 7
	}

	// 如果intervals数组为空或长度为0，则按照updateIntervalSeconds的秒数进行监控项上报
	if len(ct.intervals) == 0 {
		ct.logger.Debugf("监控项 %s intervals为空，可执行（由updateIntervalSeconds控制间隔）", ct.monitorItem.ItemName)
		return true
	}

	// 如果有自定义间隔，检查是否在范围内
	for _, interval := range ct.intervals {
		if ct.isInCustomInterval(interval, dayOfWeek, currentTime) {
			ct.logger.Debugf("监控项 %s 处于自定义间隔范围内，可执行", ct.monitorItem.ItemName)
			return true
		}
	}
	
	// 如果有自定义间隔但不在范围内，检查是否应该使用updateIntervalSeconds
	// 根据需求：如果不处于自定义时间的区间内则再按照updateIntervalSeconds的秒数进行监控项上报
	ct.logger.Debugf("监控项 %s 不在自定义间隔范围内，可使用updateIntervalSeconds执行", ct.monitorItem.ItemName)
	return true
}
