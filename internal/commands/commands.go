package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"

	"mfcbot/internal/database"
)

// Handler processes the bot's slash commands against the database.
type Handler struct {
	DB           *database.Database
	LookupID     func(ctx context.Context, name string) (int, error)
	TriggerCheck func()
}

// Handle dispatches a Discord interaction to the matching command.
func (h *Handler) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	switch data.Name {
	case "addmodel":
		h.addModel(s, i, data)
	case "updatemodel":
		h.updateModel(s, i, data)
	case "removemodel":
		h.removeModel(s, i, data)
	case "getmodelid":
		h.getModelID(s, i, data)
	case "listmodels":
		h.listModels(s, i, data)
	}
}

func opt(data discordgo.ApplicationCommandInteractionData, name string) string {
	for _, o := range data.Options {
		if o.Name == name {
			return fmt.Sprintf("%v", o.Value)
		}
	}
	return ""
}

func reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("commands: interaction reply: %v", err)
	}
}

func (h *Handler) addModel(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	model := opt(data, "model")
	modelID := opt(data, "modelid")
	message := opt(data, "message")
	channel := opt(data, "channel")
	if _, err := h.DB.AddModel(modelID, model, message, channel, 0, false); err != nil {
		reply(s, i, "Failed to add model: "+err.Error())
		return
	}
	reply(s, i, "Model "+model+" was added with the message: "+message)
	if h.TriggerCheck != nil {
		h.TriggerCheck()
	}
}

func (h *Handler) updateModel(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	model := opt(data, "model")
	modelID := opt(data, "modelid")
	message := opt(data, "message")
	channel := opt(data, "channel")
	n, err := h.DB.UpdateModel(modelID, model, message, channel, 0, false)
	if err != nil {
		reply(s, i, "Failed to update model: "+err.Error())
		return
	}
	if n > 0 {
		reply(s, i, "Model "+model+" was updated with the message: "+message)
	} else {
		reply(s, i, "Model "+model+" was not updated")
	}
}

func (h *Handler) removeModel(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	modelID := opt(data, "modelid")
	channel := opt(data, "channel")
	n, err := h.DB.DeleteModel(modelID, channel)
	if err != nil {
		reply(s, i, "Failed to remove model: "+err.Error())
		return
	}
	if n > 0 {
		reply(s, i, "Model "+modelID+" was removed")
	} else {
		reply(s, i, "Model "+modelID+" was not removed")
	}
}

func (h *Handler) getModelID(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	name := opt(data, "modelname")
	id, err := h.LookupID(context.Background(), name)
	if err != nil {
		reply(s, i, "Model ID lookup failed: "+err.Error())
		return
	}
	reply(s, i, fmt.Sprintf("Model ID is %d", id))
}

func (h *Handler) listModels(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	channel := opt(data, "channel")
	models, err := h.DB.GetModels()
	if err != nil {
		reply(s, i, "Failed to list models: "+err.Error())
		return
	}
	var sb strings.Builder
	for _, m := range models {
		if m.Channel == channel {
			sb.WriteString(m.ModelName + " : " + m.ModelID + ", ")
		}
	}
	if sb.Len() == 0 {
		reply(s, i, "No models are being watched")
		return
	}
	reply(s, i, sb.String())
}
