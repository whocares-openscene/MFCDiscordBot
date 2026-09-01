import fetch from 'node-fetch';
import { title } from 'process';
import * as cheerio from 'cheerio';
import axios from 'axios';
import fs from 'fs';


export async function gettopic(modelid) {
    var url = "https://api-edge.myfreecams.com/recommend?model_id=" + modelid +  "&version2=1&=";
    try {
        const response = await fetch(url);
        const body = await response.json();
        return body['result']['users'][modelid]['room_topic'];    
    } catch (error) {
        console.log(error);
        return false
    }
}

export async function getmodelusername(modelid) {
    ////console.log("function");
    var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelid;
    try {
        const response = await fetch(url);
        const body = await response.json();
        if (body['result']['success'] === 0) {
            return "Lookup failed"
        }
        return body['result']['user']['username'];
    } catch (error) {
        console.log(error);
        return "Lookup failed";
    }
    ////console.log(result[1]);
}

export async function getmodelid(modelname) {
    ////console.log("function");
    var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelname;
    try {
        const response = await fetch(url);
        const body = await response.json();
        if (body['result']['success'] === 0) {
            return "Lookup failed"
        }
        return body['result']['user']['id'];
    } catch (error) {
        console.log(error);
        return "Lookup failed";
    }
    ////console.log(result[1]);
}

export async function getPicture(url, modelid) {
    ///const ts = Date.now();
    const location = import.meta.dirname + '/' + modelid + '.jpg';
    ///const server = await getstatus(modelid);
    ///var url = "https://snap.mfcimg.com/snapimg/" + server['server'] + "/341x192/mfc_1" + modelid;
    await downloadImage(url, location);
    return location;
}

async function downloadImage(url, filepath) {
  const response = await axios({
    url,
    method: 'GET',
    responseType: 'stream'
  });

  return new Promise((resolve, reject) => {
    const writer = fs.createWriteStream(filepath);
    response.data.pipe(writer);
    
    writer.on('finish', resolve);
    writer.on('error', reject);
  });
}


/*
export async function getCalendar(modelname) {
    var url = "https://share.myfreecams.com/" + modelname + "/calendar?list_view=true";
    try {
        const response = await fetch(url);
        const body = await response.text();
        if (response['status'] == 200) {
            const events = [];
            const $ = cheerio.load(body);
            const htmlEvents = $('current-month has-events');
            console.log($);
            for (let i = 0; i < htmlEvents.length; i++) {
                const element = htmlEvents[i];
                const event = {
                    date: $(element).find("day-header")[0].text(),
                    time: $(element).find("start-time")[0].text(),
                    title: $(element).find("title")[0].text()
                };
            }
            return events;
        }
    } catch (error) {
        console.log(error);
        return "Lookup failed";
    }
}
    */

export async function getstatus(modelid) {

    //console.log("Entered getstatus" + modelid);

    var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelid;
    const modelstatus = {
        status: 1,
        topic: "",
        username: "",
        server: ''
    }
    //console.log(modelstatus);
    try {
        const response = await fetch(url);
        const body = await response.json();
        //console.log("Try");
        if (body['result']['success'] === 0) {
            //console.log("o/home/joeln/MFCBot/MFCDiscordBotffline");
            //console.log(body['result']['message'])
            //console.log(modelstatus);
            //console.log("1");
            return modelstatus;
        }
        
        if (0 in body['result']['user']['sessions']) {
            //console.log("Online");
            let status = body['result']['user']['sessions'][0];
            //console.log("modelstatus");
            if (status['vidserver_id'] == 0) {
                return modelstatus;
            }
            //console.log(status['server_name'].match(/(\d+)/)[0]);
            modelstatus['server'] = status['server_name'].match(/(\d)+./)[0];
            modelstatus['status'] = status['vstate'];
            modelstatus['topic'] = status['room_topic'];
            modelstatus['username'] = body['result']['user']['username'];
            //console.log(modelstatus);
            return modelstatus;
        } else {
            modelstatus['username'] = body['result']['user']['username'];
            return modelstatus;
        }

        ////console.log(body);
    } catch (error) {
        //console.log("Try Fail");
        console.log(error);
        return modelstatus;
    }
}

export async function getallonline() {
    //console.log("running all online")
    var url = "http://mfcgo/online";
    try {
        const response = await fetch(url);
        const body = await response.json();
        const count = Object.keys(body.streamers).length;
        if (count > 0) {
            console.log(count);
            return body.streamers
        }
    } catch (error) {
        //console.log("Try Fail");
        console.log(error);
        return;
    }
    
}




/*
13 - group
0 - online
12 - private
14 - club


https://api-edge.myfreecams.com/usernameLookup/mollymayhem

*/
