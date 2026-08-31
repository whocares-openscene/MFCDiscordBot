import fetch from 'node-fetch';
import { title } from 'process';
import * as cheerio from 'cheerio';
import axios from 'axios';
import fs from 'fs';

const options = {
  method: 'GET',
  headers: {
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Accept': 'application/json',
    'Cookie': 'pref_cli=w; baf=17686577664212444; color_scheme_dark_embed=true; _mfcs_perm=eyJfcmFpbHMiOnsibWVzc2FnZSI6IklqTmlNRGcxTVdabExUUTBaVGN0TkdFME15MWlNVGs1TFdWa1pXTTBObVEzTldReU9TST0iLCJleHAiOm51bGwsInB1ciI6ImNvb2tpZS5fbWZjc19wZXJtIn19--1bd919f83a1a09b5f541c86bf8bb090385270bec; window_size=1696x1043; _mfcs_session_v2=tY7uU63wSjqsUn25F2AQAJ8Y8Z%2B8yxKhGVEl2jfQ1r%2B82nPGqKyaobX%2FDdo7vsm%2BtE%2BJT%2BhbrSmTHQiMETOZKN7Epxx9ve1QF9glc9Sn8TY5IeHKTbnzaHD2CPnwUgZhounGurDXc14G3CvZbBNxqu3AsrrJZJR0dmxp58jd8fslzaWqu9iEMFbBpoRi%2BE%2BLlSEFuaDTiP9r37oevcL7CjUM%2FWJi3GanbUQj1FSm7oKUOY0fjwx0DEar0ue8A5U88MWsEDbu0QgejPWPISUKsL267c2HS4egdDHm3xKls2iY9HTU6UQYaRHeyoSzqA%3D%3D--oINqeqPD5BuHtsbR--972WzUVy3yzlhhNVJ0Le4w%3D%3D; g_state={"i_l":1,"i_ll":1787624231195,"i_b":"ajg3EDoYglc29HfU2IEkG125fGPRPejyl6v/ci4yjbA","i_e":{"enable_itp_optimization":24},"i_et":1787624231195}; ref=https%3A%2F%2Fshare.myfreecams.com%2F; browser_time_zone=America/New_York; ref_model_id=29845158; gw=true; calendar_filter_list_view=true; _gcl_au=1.1.1432150834.1787623582'
  }
};


export async function gettopic(modelid) {
    var url = "https://api-edge.myfreecams.com/recommend?model_id=" + modelid +  "&version2=1&=";
    try {
        const response = await fetch(url, options);
        const body = await response.json();
        return body['result']['users'][modelid]['room_topic'];    
    } catch (error) {
        console.log(error);
        return false
    }
}

export async function getmodelid(modelname) {
    ////console.log("function");
    var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelname;
    try {
        const response = await fetch(url, options);
        const body = await response.json();
        return body['result']['user']['id'];
    } catch (error) {
        console.log(error);
        return "Lookup failed";
    }
    ////console.log(result[1]);
}

export async function getPicture(modelid) {
    const ts = Date.now();
    const location = import.meta.dirname + '/' + modelid + '.jpg';
    const server = await getstatus(modelid);
    var url = "https://snap.mfcimg.com/snapimg/" + server['server'] + "/341x192/mfc_1" + modelid;
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
        const response = await fetch(url, options);
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
        const response = await fetch(url, options);
        const body = await response.json();
        //console.log("Try");
        if (body['result']['success'] === 0) {
            //console.log("offline");
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




/*
13 - group
0 - online
12 - private
14 - club


https://api-edge.myfreecams.com/usernameLookup/mollymayhem

*/