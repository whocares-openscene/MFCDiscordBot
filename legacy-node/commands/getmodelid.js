import {
  SlashCommandBuilder,
  PermissionFlagsBits,
  MessageFlags,
} from "discord.js";
import * as webstuff from "../webstuff.js";

// Command Builder export
export const data = new SlashCommandBuilder()
  .setName("getmodelid")
  .setDescription("Gets a Model ID value")
  .addStringOption((option) =>
    option
      .setName("modelname")
      .setDescription("Model's username")
      .setRequired(true),
  )
  .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels);

// Execute function export
export async function execute(interaction) {
  //console.log("interaction");
  const modelname = interaction.options.getString("modelname");
  const modelid = await webstuff.getmodelid(modelname);
  await interaction.reply({
    content: "Model ID is " + modelid,
    flags: [MessageFlags.Ephemeral],
  });
}
