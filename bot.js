import { Client, Events, GatewayIntentBits } from 'discord.js';
import { config } from 'dotenv';
import * as addmodel from './commands/addmodel.js';
import * as updatemodel from './commands/updatemodel.js';
import * as removemodel from './commands/removemodel.js';
import * as dbfunctions from './databasefunctions.js';
import * as getmodelid from './commands/getmodelid.js';
import * as webstuff from './webstuff.js';
import * as listmodels from './commands/listmodels.js';
import { unlink } from 'node:fs/promises';

config();

// Create a new Discord client instance
const client = new Client({
    intents: [GatewayIntentBits.Guilds],
});

// Handle interactions with the Discord client
async function handleInteraction(interaction) {
    if (!interaction.isCommand()) return;
    if (interaction.commandName === 'addmodel') {
        await addmodel.execute(interaction);
        await onlinecheck();
    } else if (interaction.commandName === "updatemodel") {
        await updatemodel.execute(interaction);
    } else if (interaction.commandName === "removemodel") {
        await removemodel.execute(interaction);
    } else if (interaction.commandName === "getmodelid") {
        await getmodelid.execute(interaction);
    } else if (interaction.commandName === "listmodels") {
        await listmodels.execute(interaction);
    }
}

// When the client is ready, run this code (only once)
client.once(Events.ClientReady, readyDiscord);

// Login to Discord with your client's token
client.login(process.env.TOKEN);

async function readyDiscord() {
    console.log('💖 v5');
    const models = await dbfunctions.getmodels();
    for (let index = 0; index < models.length; index++) {
        const element = models[index];
        await dbfunctions.updatetopic(element['modelid'], false, element['channel']);
    }
    onlinecheck();
    setInterval(onlinecheck, 120000);
}

client.on(Events.InteractionCreate, handleInteraction);

// Main function to check online status of models
async function onlinecheck() {
    const models = await dbfunctions.getmodels();
    const allmodels = await webstuff.getallonline();
    if (typeof allmodels == "undefined") {
        return
    }
    for (let index = 0; index < models.length; index++) {
        const element = models[index];
        const name = await webstuff.getmodelusername(element['modelid']);
        const channel = client.channels.cache.get(element['channel']);
        const ts = Date.now() - element['time'];
        var time = new Date(ts);
        element['int'] = parseInt(element['topic']);
        if (Object.hasOwn(allmodels, name)) {
          const modelstatus = allmodels[name];
          if (modelstatus['show_kind'] === 2 && element['topic'] != 2) {
              // Group show
                const message = element['modelname'] + " is now in a group show!";
                await channel.send(message);
              await dbfunctions.updatetopic(element['modelid'], modelstatus['show_kind'], element['channel']);
            } else if (modelstatus['show_kind'] === 3 && element['topic'] != 3) {
              // Club show
                const message = element['modelname'] + " is now in a club show!";
                await channel.send(message);
                await dbfunctions.updatetopic(element['modelid'], modelstatus['show_kind'], element['channel']);
            } else if ((Number.isInteger(element['int']) || element['int'] === false) && modelstatus['show_kind'] === 1  && (typeof modelstatus['subject'] != "undefined")) {
                // Public show
                const image = await webstuff.getPicture(modelstatus['image_url'], element['modelid']);
                const message = element['message'] + "\nCurrent topic is:\n" + modelstatus['subject'];
                await channel.send({
                    content: message,
                    files: [image]
                });
                await dbfunctions.updatetopic(element['modelid'], modelstatus['subject'], element['channel']);
                const time = Date.now();
                await dbfunctions.updatetime(element['modelid'], time, element['channel']);
                deleteFile(image);
            }
        } else if (element['topic'] != false) {
            //offline
            const message = element['modelname'] + " has gone offline. She was online for " + time.getUTCHours() + " hours " + time.getUTCMinutes() + " minutes and " + time.getUTCSeconds() + " seconds";
            await channel.send(message);
            await dbfunctions.updatetime(element['modelid'], false, element['channel']);
            await dbfunctions.updatetopic(element['modelid'], false, element['channel']);
        } else if (element['time'] != false) {
            //offline while bot gone
            const message = element['modelname'] + " has gone offline while the bot was offline.";
            await channel.send(message);
            await dbfunctions.updatetime(element['modelid'], false, element['channel']);
            await dbfunctions.updatetopic(element['modelid'], false, element['channel']);
        }

async function deleteFile(filepath) {
  try {
    await unlink(filepath);
  } catch (error) {
    console.error('Error deleting file:', error.message);
  }
}

/*
show_kind
const (
	// ShowUnknown means the show kind is unknown
	ShowUnknown ShowKind = 0
	// ShowPublic means the show is public
	ShowPublic ShowKind = 1
	// ShowGroup means the show is a group show
	ShowGroup ShowKind = 2
	// ShowTicket means the show is a ticket show
	ShowTicket ShowKind = 3
	// ShowHidden means the show is hidden
	ShowHidden ShowKind = 4
	// ShowPrivate means the show is private
	ShowPrivate ShowKind = 5
	// ShowAway means the model is away
	ShowAway ShowKind = 6
)
    */
