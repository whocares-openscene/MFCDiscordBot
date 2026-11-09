import {
  SlashCommandBuilder,
  PermissionFlagsBits,
  MessageFlags,
} from "discord.js";
import * as db from "../databasefunctions.js";

// Command Builder export
export const data = new SlashCommandBuilder()
  .setName("updatemodel")
  .setDescription("Updates a model to watch on the server")
  .addStringOption((option) =>
    option
      .setName("model")
      .setDescription("Model Name to watch")
      .setRequired(true),
  )
  .addStringOption((option) =>
    option
      .setName("modelid")
      .setDescription("Model ID number")
      .setRequired(true),
  )
  .addStringOption((option) =>
    option
      .setName("message")
      .setDescription("Message to announce")
      .setRequired(true),
  )
  .addChannelOption((option) =>
    option
      .setName("channel")
      .setDescription("The channel to update")
      .setRequired(true),
  )
  .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels);

// Execute function export
export async function execute(interaction) {
  const modelid = interaction.options.getString("modelid");
  const model = interaction.options.getString("model");
  const message = interaction.options.getString("message");
  const channel = interaction.options.getChannel("channel").id;
  const result = await db.updatemodel(
    modelid,
    model,
    message,
    channel,
    0,
    false,
  );
  if (result > 0) {
    await interaction.reply({
      content: "Model " + model + " was updated with the message: " + message,
      flags: [MessageFlags.Ephemeral],
    });
  } else {
    await interaction.reply({
      content: "Model " + model + " was not updated",
      flags: [MessageFlags.Ephemeral],
    });
  }
  //await interaction.reply(interaction.options.getChannel('channel'))
}
