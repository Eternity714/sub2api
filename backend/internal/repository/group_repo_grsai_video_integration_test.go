//go:build integration

package repository

import "github.com/Wei-Shaw/sub2api/internal/service"

func (s *GroupRepoSuite) TestCreatePreservesGrsaiVideoPrices() {
	prices := map[string]map[string]float64{
		"minimax-h3": {"480p": 0.10, "768p": 0.14, "1080p": 0.30},
	}
	group := &service.Group{
		Name: "grsai-video-create", Platform: service.PlatformGrsai,
		Status: service.StatusActive, RateMultiplier: 1,
		SubscriptionType: service.SubscriptionTypeStandard, VideoModelPrices: prices,
	}
	s.Require().NoError(s.repo.Create(s.ctx, group))
	stored, err := s.tx.Client().Group.Get(s.ctx, group.ID)
	s.Require().NoError(err)
	s.Require().Equal(prices, stored.VideoModelPrices)
}

func (s *GroupRepoSuite) TestNameOnlyUpdatePreservesGrsaiVideoPrices() {
	prices := map[string]map[string]float64{
		"minimax-h3": {"480p": 0.10, "768p": 0.14, "1080p": 0.30},
	}
	group := &service.Group{
		Name: "grsai-video-rename", Platform: service.PlatformGrsai,
		Status: service.StatusActive, RateMultiplier: 1,
		SubscriptionType: service.SubscriptionTypeStandard,
	}
	s.Require().NoError(s.repo.Create(s.ctx, group))
	// Seed the raw stored map so the test also catches lossy reads before Update.
	_, err := s.tx.Client().Group.UpdateOneID(group.ID).SetVideoModelPrices(prices).Save(s.ctx)
	s.Require().NoError(err)
	loaded, err := s.repo.GetByID(s.ctx, group.ID)
	s.Require().NoError(err)
	loaded.Name = "媒体测试分组"
	s.Require().NoError(s.repo.Update(s.ctx, loaded))
	stored, err := s.tx.Client().Group.Get(s.ctx, group.ID)
	s.Require().NoError(err)
	s.Require().Equal("媒体测试分组", stored.Name)
	s.Require().Equal(prices, stored.VideoModelPrices)
}
