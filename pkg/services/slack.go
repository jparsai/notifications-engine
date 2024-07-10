package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	texttemplate "text/template"

	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	slackutil "github.com/argoproj/notifications-engine/pkg/util/slack"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

// No rate limit unless Slack requests it (allows for Slack to control bursting)
var slackState = slackutil.NewState(rate.NewLimiter(rate.Inf, 1))

type SlackNotification struct {
	Attachments     string                   `json:"attachments,omitempty"`
	Blocks          string                   `json:"blocks,omitempty"`
	GroupingKey     string                   `json:"groupingKey"`
	NotifyBroadcast bool                     `json:"notifyBroadcast"`
	DeliveryPolicy  slackutil.DeliveryPolicy `json:"deliveryPolicy"`
}

func (n *SlackNotification) GetTemplater(name string, f texttemplate.FuncMap) (Templater, error) {
	fmt.Println("######## Slack_GetTemplater")

	slackAttachments, err := texttemplate.New(name).Funcs(f).Parse(n.Attachments)
	if err != nil {
		fmt.Println("######## Slack_GetTemplater_1_error = ", err)
		return nil, err
	}
	slackBlocks, err := texttemplate.New(name).Funcs(f).Parse(n.Blocks)
	if err != nil {
		fmt.Println("######## Slack_GetTemplater_2_error", err)
		return nil, err
	}
	groupingKey, err := texttemplate.New(name).Funcs(f).Parse(n.GroupingKey)
	if err != nil {
		fmt.Println("######## Slack_GetTemplater_3_error")
		return nil, err
	}

	return func(notification *Notification, vars map[string]interface{}) error {
		fmt.Println("######## Slack_GetTemplater_4_error")

		if notification.Slack == nil {
			notification.Slack = &SlackNotification{}
		}
		var slackAttachmentsData bytes.Buffer
		if err := slackAttachments.Execute(&slackAttachmentsData, vars); err != nil {
			fmt.Println("######## Slack_GetTemplater_5_error")
			return err
		}
		notification.Slack.Attachments = slackAttachmentsData.String()

		var slackBlocksData bytes.Buffer
		if err := slackBlocks.Execute(&slackBlocksData, vars); err != nil {
			fmt.Println("######## Slack_GetTemplater_6_error")
			return err
		}
		notification.Slack.Blocks = slackBlocksData.String()

		var groupingKeyData bytes.Buffer
		if err := groupingKey.Execute(&groupingKeyData, vars); err != nil {
			fmt.Println("######## Slack_GetTemplater_7_error")
			return err
		}
		notification.Slack.GroupingKey = groupingKeyData.String()

		notification.Slack.NotifyBroadcast = n.NotifyBroadcast
		notification.Slack.DeliveryPolicy = n.DeliveryPolicy
		fmt.Println("######## Slack_GetTemplater_8")
		return nil
	}, nil
}

type SlackOptions struct {
	Username           string   `json:"username"`
	Icon               string   `json:"icon"`
	Token              string   `json:"token"`
	SigningSecret      string   `json:"signingSecret"`
	Channels           []string `json:"channels"`
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`
	ApiURL             string   `json:"apiURL"`
	DisableUnfurl      bool     `json:"disableUnfurl"`
}

type slackService struct {
	opts SlackOptions
}

var validIconEmoji = regexp.MustCompile(`^:.+:$`)

func NewSlackService(opts SlackOptions) NotificationService {
	return &slackService{opts: opts}
}

func buildMessageOptions(notification Notification, dest Destination, opts SlackOptions) (*SlackNotification, []slack.MsgOption, error) {
	fmt.Println("######## Slack_buildMessageOptions")

	msgOptions := []slack.MsgOption{slack.MsgOptionText(notification.Message, false)}
	slackNotification := &SlackNotification{}

	if opts.Username != "" {
		msgOptions = append(msgOptions, slack.MsgOptionUsername(opts.Username))
	}
	if opts.Icon != "" {
		if validIconEmoji.MatchString(opts.Icon) {
			msgOptions = append(msgOptions, slack.MsgOptionIconEmoji(opts.Icon))
		} else if isValidIconURL(opts.Icon) {
			msgOptions = append(msgOptions, slack.MsgOptionIconURL(opts.Icon))
		} else {
			log.Warnf("Icon reference '%v' is not a valid emoji or url", opts.Icon)
		}
	}
	if notification.Slack != nil {
		fmt.Println("######## Slack_buildMessageOptions_1")
		attachments := make([]slack.Attachment, 0)
		if notification.Slack.Attachments != "" {
			if err := json.Unmarshal([]byte(notification.Slack.Attachments), &attachments); err != nil {
				fmt.Println("######## Slack_buildMessageOptions_2_error = ", err)
				return nil, nil, fmt.Errorf("failed to unmarshal attachments '%s' : %v", notification.Slack.Attachments, err)
			}
		}

		blocks := slack.Blocks{}
		if notification.Slack.Blocks != "" {
			if err := json.Unmarshal([]byte(notification.Slack.Blocks), &blocks); err != nil {
				fmt.Println("######## Slack_buildMessageOptions_3_error")
				return nil, nil, fmt.Errorf("failed to unmarshal blocks '%s' : %v", notification.Slack.Blocks, err)
			}
		}
		fmt.Println("######## Slack_buildMessageOptions_4")
		msgOptions = append(msgOptions, slack.MsgOptionAttachments(attachments...), slack.MsgOptionBlocks(blocks.BlockSet...))
		slackNotification = notification.Slack
	}

	if opts.DisableUnfurl {
		msgOptions = append(msgOptions, slack.MsgOptionDisableLinkUnfurl(), slack.MsgOptionDisableMediaUnfurl())
	}
	fmt.Println("######## Slack_buildMessageOptions_5")
	return slackNotification, msgOptions, nil
}

func (s *slackService) Send(notification Notification, dest Destination) error {
	fmt.Println("######## Slack_Send")

	slackNotification, msgOptions, err := buildMessageOptions(notification, dest, s.opts)
	if err != nil {
		fmt.Println("######## Slack_Send_1")
		return err
	}
	return slackutil.NewThreadedClient(
		newSlackClient(s.opts),
		slackState,
	).SendMessage(
		context.TODO(),
		dest.Recipient,
		slackNotification.GroupingKey,
		slackNotification.NotifyBroadcast,
		slackNotification.DeliveryPolicy,
		msgOptions,
	)
}

// GetSigningSecret exposes signing secret for slack bot
func (s *slackService) GetSigningSecret() string {
	return s.opts.SigningSecret
}

func newSlackClient(opts SlackOptions) *slack.Client {
	fmt.Println("######## Slack_newSlackClient")

	apiURL := slack.APIURL
	if opts.ApiURL != "" {
		apiURL = opts.ApiURL
	}
	transport := httputil.NewTransport(apiURL, opts.InsecureSkipVerify)
	client := &http.Client{
		Transport: httputil.NewLoggingRoundTripper(transport, log.WithField("service", "slack")),
	}
	return slack.New(opts.Token, slack.OptionHTTPClient(client), slack.OptionAPIURL(apiURL))
}

func isValidIconURL(iconURL string) bool {
	fmt.Println("######## Slack_isValidIconURL")

	_, err := url.ParseRequestURI(iconURL)
	if err != nil {
		return false
	}

	u, err := url.Parse(iconURL)
	if err != nil || (u.Scheme == "" || !(u.Scheme == "http" || u.Scheme == "https")) || u.Host == "" {
		return false
	}

	return true
}
