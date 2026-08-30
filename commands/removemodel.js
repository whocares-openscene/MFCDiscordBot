import { SlashCommandBuilder, PermissionFlagsBits, MessageFlags } from 'discord.js';
import * as db from '../databasefunctions.js';


// Command Builder export
export const data = new SlashCommandBuilder()
    .setName('removemodel')
    .setDescription('Removes a watched model')
    .addStringOption(option =>
        option.setName('modelid')
            .setDescription('Model ID number')
            .setRequired(true))
    .addChannelOption(option =>
        option.setName('channel')
            .setDescription('The channel to post into')
            .setRequired(true))
    .setDefaultMemberPermissions(PermissionFlagsBits.ManageChannels);


// Execute function export
export async function execute(interaction) {
    const modelid = interaction.options.getString('modelid');
    const channel = interaction.options.getChannel('channel').id;
    //console.log(server);
    const result = await db.deletemodel(modelid, channel);
    if (result == 1) {
        await interaction.reply({content: "Model " + modelid + " was removed", flags: [MessageFlags.Ephemeral]});    
    } else {
        await interaction.reply({content: "Model " + modelid + " was not removed", flags: [MessageFlags.Ephemeral]});
    }
    //await interaction.reply(interaction.options.getChannel('channel'))
    
}