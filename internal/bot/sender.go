package bot

import (
	"context"
	"time"

	"golang.org/x/time/rate"
	tele "gopkg.in/telebot.v4"
)

type Sender struct {
	bot     *tele.Bot
	limiter *rate.Limiter
}

func NewSender(b *tele.Bot, perSecond int) *Sender {
	if perSecond <= 0 {
		perSecond = 5
	}
	return &Sender{bot: b, limiter: rate.NewLimiter(rate.Limit(perSecond), perSecond)}
}

func (s *Sender) SendOwner(ctx context.Context, ownerID int64, text string, opts ...interface{}) (*tele.Message, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.limiter.Wait(waitCtx); err != nil {
		return nil, err
	}
	return s.bot.Send(&tele.User{ID: ownerID}, text, opts...)
}

func (s *Sender) CopyOwner(ctx context.Context, ownerID int64, msg tele.Editable, opts ...interface{}) (*tele.Message, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.limiter.Wait(waitCtx); err != nil {
		return nil, err
	}
	return s.bot.Copy(&tele.User{ID: ownerID}, msg, opts...)
}

func (s *Sender) SendUser(userID int64, text string, opts ...interface{}) (*tele.Message, error) {
	return s.bot.Send(&tele.User{ID: userID}, text, opts...)
}

func (s *Sender) CopyUser(userID int64, msg tele.Editable, opts ...interface{}) (*tele.Message, error) {
	return s.bot.Copy(&tele.User{ID: userID}, msg, opts...)
}
