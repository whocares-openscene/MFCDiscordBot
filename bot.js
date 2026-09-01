import { Client, Events, GatewayIntentBits, SlashCommandRoleOption } from 'discord.js';
import { config } from 'dotenv';
import * as addmodel from './commands/addmodel.js';
import * as updatemodel from './commands/updatemodel.js';
import * as removemodel from './commands/removemodel.js';
import * as dbfunctions from './databasefunctions.js';
import * as getmodelid from './commands/getmodelid.js';
import * as webstuff from './webstuff.js';
import * as listmodels from './commands/listmodels.js';
import * as intervaldatabase from './intervaldatabase.js';
import { unlink } from 'node:fs/promises';
import { all } from 'axios';


config();

// Create a new client instance
const client = new Client({
    intents: [GatewayIntentBits.Guilds],
});

async function handleInteraction(interaction) {
    if (!interaction.isCommand()) return;
    if (interaction.commandName === 'addmodel') {
        await addmodel.execute(interaction);
        await onlinecheck();
    } else if (interaction.commandName === "updatemodel") {
        await updatemodel.execute(interaction);
    } else if (interaction.commandName === "removemodel") {
        await removemodel.execute(interaction);
        //await removeinterval(interaction.options.getString('modelid'), interaction.options.getChannel('channel').id);
    } else if (interaction.commandName === "getmodelid") {
        await getmodelid.execute(interaction);
    } else if (interaction.commandName === "listmodels") {
        await listmodels.execute(interaction);
    }
}


async function sendmessage(channelid, message) {
    //console.log(message);
    var channel = client.channels.cache.get(channelid);
    await channel.send(message);

}

// When the client is ready, run this code (only once)
client.once(Events.ClientReady, readyDiscord);

// Login to Discord with your client's token
client.login(process.env.TOKEN);

async function readyDiscord() {
    console.log('💖 v4');
    const models = await dbfunctions.getmodels();
    for (let index = 0; index < models.length; index++) {
        const element = models[index];
        await dbfunctions.updatetopic(element['modelid'], false, element['channel']);
    }
    //onlinecheck();
    setInterval(onlinecheck, 30000);
}

client.on(Events.InteractionCreate, handleInteraction);

async function onlinecheck() {
    //console.log("Online")
    const models = await dbfunctions.getmodels();
    const skipstatus = [0,2,3,4,5,6];
    const allmodels = await webstuff.getallonline();
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
                const message = element['modelname'] + " is now in a group show!";
                await channel.send(message);
                await dbfunctions.updatetopic(element['modelid'], modelstatus['show_kind'], element['channel']);
            } else if(modelstatus['show_kind'] === 3 && element['topic'] != 3) { // Club
                const message = element['modelname'] + " is now in a club show!";
                await channel.send(message);
                await dbfunctions.updatetopic(element['modelid'], modelstatus['show_kind'], element['channel']);
            } else if ((Number.isInteger(element['int']) || element['int'] === false) && modelstatus['show_kind'] === 1  && modelstatus['topic'] != "") { 
                console.log("online")
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
        } else if (element['time'] != false) {
            //offline while bot gone
            const message = element['modelname'] + " has gone offline while the bot was offline.";
            await channel.send(message);
            await dbfunctions.updatetime(element['modelid'], false, element['channel']);
            await dbfunctions.updatetopic(element['modelid'], false, element['channel']);
        } else if (element['topic'] != false) {
            const message = element['modelname'] + " has gone offline. She was online for " + time.getUTCHours() + " hours " + time.getUTCMinutes() + " minutes and " + time.getUTCSeconds() + " seconds";
            await channel.send(message);
            await dbfunctions.updatetime(element['modelid'], false, element['channel']);
            await dbfunctions.updatetopic(            console.log(modelstatus);element['modelid'], false, element['channel']);
        }
        //const modelstatus = await webstuff.getstatus(element['modelid']);
        /*
        if(modelstatus['status'] === 13 && element['topic'] != 13) { // Group            
            const message = element['modelname'] + " is now in a group show!";
            await channel.send(message);
            await dbfunctions.updatetopic(element['modelid'], modelstatus['status'], element['channel']);
        } else if(modelstatus['status'] === 14 && element['topic'] != 14) { // Club
            const message = element['modelname'] + " is now in a club show!";
            await channel.send(message);
            await dbfunctions.updatetopic(element['modelid'], modelstatus['status'], element['channel']);
        } else if ((Number.isInteger(element['int']) || element['int'] === false) && !skipstatus.includes(modelstatus['status'])  && modelstatus['topic'] != "") { 
            const image = await webstuff.getPicture(element['modelid']);
            const message = element['message'] + "\nCurrent topic is:\n" + modelstatus['topic'];

            await channel.send({ 
                content: message, 
                files: [image]
            });
            await dbfunctions.updatetopic(element['modelid'], modelstatus['topic'], element['channel']);
            const time = Date.now();
            await dbfunctions.updatetime(element['modelid'], time, element['channel']);
            deleteFile(image);
            
        } else if (modelstatus['status'] === 1 && element['time'] != false) { // offline
            const ts = Date.now() - element['time'];
            
        }
        /*if (element['events'] == true) {
            updateEvents(channel['guildId'], modelstatus['username']);
        }*/


        /*
        
        
        else if (modelstatus['status'] && element['topic'] == false) {
            
            const message = element['message'] + "\nCurrent topic is:\n" + currenttopic;
            //await channel.send(element['message']);
            //await channel.send("Current topic is:");
            //await channel.send(currenttopic);
            await channel.send(message);
            await dbfunctions.updatetopic(element['modelid'], currenttopic, element['channel']);
            const time = Date.now();
            await dbfunctions.updatetime(element['modelid'], time, element['channel']);
            if (Number(element['update']) > 5) {
                const time = Number(element['update']) * 60 * 1000
                //console.log(time);
                const interval = setInterval(topicchange, time, element['modelid'], element['channel']);
                const intervalId = interval[Symbol.toPrimitive]();
                //console.log(intervalId);
                await intervaldatabase.addinterval(element['modelid'], element['channel'], intervalId);
            }
        } else if (!currenttopic && element['time'] != false) {
            
            const ts = Date.now() - element['time'];
            var time = new Date(ts);
            const message = element['modelname'] + " has gone offline while the bot was offline.";
            await channel.send(message);
            await dbfunctions.updatetime(element['modelid'], false, element['channel']);
            await dbfunctions.updatetopic(element['modelid'], false), element['channel'];
        }
        */
    }
}
/*
async function updateEvents(guildId, username) {

    const events = await webstuff.getCalendar(guildId, username);
    console.log(guildId + username);
    console.log(events);
    var time = "";
    
    
}*/

/*
13 - group
0 - online
12 - private
14 - club

async function topicchange(modelid, channel) {
    //console.log("Topic");
    //const interval = await intervaldatabase.getinterval(modelid, channel);
    //console.log(interval);
    const model = await dbfunctions.getmodel(modelid, channel);
    const currenttopic = await webstuff.gettopic(modelid);
    try {groupStatus.includes(modelstatus['show_kind']) && !groupStatus.includes(element['topic'])
        if (model[0]['update'] === false) {
            clearInterval(this)
        } else if (!currenttopic) {
            clearInterval(this);
        } else if (currenttopic != model[0]['topic']) {
            const channel = client.channels.cache.get(model[0]['channel']);
            await channel.send("New topic for " + model[0]['modelname'] + "\n" + currenttopic);
            //await channel.send(currenttopic);
            dbfunctions.updatetopic(modelid, currenttopic, channel);
        }   
    } catch (error) {
        clearInterval(this);
        console.log("Change fail " + modelid + ", " + channel);
    }
}
     */

async function removeinterval(modelid, channel) {
    try {
        const interval = await intervaldatabase.getinterval(modelid, channel);
        clearInterval(interval);
        await intervaldatabase.deleteinterval(modelid, channel);
    } catch (error) {
        console.log("Interval fail");
    }
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


//clearInterval(this);
