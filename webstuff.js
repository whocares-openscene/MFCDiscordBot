import fetch from "node-fetch";
import axios from "axios";
import fs from "fs";

// Get the topic for a given model ID from the MyFreeCams API
// No longer in use
export async function gettopic(modelid) {
  var url =
    "https://api-edge.myfreecams.com/recommend?model_id=${modelid}&version2=1&=";
  try {
    const response = await fetch(url);
    const body = await response.json();
    return body["result"]["users"][modelid]["room_topic"];
  } catch (error) {
    console.log(error);
    return false;
  }
}

// Get the username for a given model ID from the MyFreeCams API
export async function getmodelusername(modelid) {
  var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelid;
  try {
    const response = await fetch(url);
    const body = await response.json();
    if (body["result"]["success"] === 0) {
      return "Lookup failed";
    }
    const name = body["result"]["user"]["username"];
    return name.toLowerCase();
  } catch (error) {
    console.log(error);
    return "Lookup failed";
  }
}

// Get the model ID for a given model name from the MyFreeCams API
export async function getmodelid(modelname) {
  var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelname;
  try {
    const response = await fetch(url);
    const body = await response.json();
    if (body["result"]["success"] === 0) {
      return "Lookup failed";
    }
    return body["result"]["user"]["id"];
  } catch (error) {
    console.log(error);
    return "Lookup failed";
  }
}

// Get the live image for a model from MyFreeCams
export async function getPicture(url, modelid) {
  const location = import.meta.dirname + "/${modelid}.jpg";
  await downloadImage(url, location);
  return location;
}

// Download an image from a URL and save it to a file
async function downloadImage(url, filepath) {
  const response = await axios({
    url,
    method: "GET",
    responseType: "stream",
  });

  return new Promise((resolve, reject) => {
    const writer = fs.createWriteStream(filepath);
    response.data.pipe(writer);

    writer.on("finish", resolve);
    writer.on("error", reject);
  });
}

/*
// Attempt to get the calendar for a model from the MyFreeCams API
// Failed due to too many requests
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

// Get status of a model from the MyFreeCams API
// Not in use
export async function getstatus(modelid) {
  var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelid;
  const modelstatus = {
    status: 1,
    topic: "",
    username: "",
    server: "",
  };
  try {
    const response = await fetch(url);
    const body = await response.json();
    if (body["result"]["success"] === 0) {
      return modelstatus;
    }

    if (0 in body["result"]["user"]["sessions"]) {
      let status = body["result"]["user"]["sessions"][0];
      if (status["vidserver_id"] == 0) {
        return modelstatus;
      }
      modelstatus["server"] = status["server_name"].match(/(\d)+./)[0];
      modelstatus["status"] = status["vstate"];
      modelstatus["topic"] = status["room_topic"];
      modelstatus["username"] = body["result"]["user"]["username"];
      return modelstatus;
    } else {
      modelstatus["username"] = body["result"]["user"]["username"];
      return modelstatus;
    }
  } catch (error) {
    console.log(error);
    return modelstatus;
  }
}

// Get all online streamers from mfcgo docker container
export async function getallonline() {
  var url = "http://mfcgo/online";
  try {
    const response = await fetch(url);
    const body = await response.json();
    const count = Object.keys(body.streamers).length;
    if (count > 0) {
      return body.streamers;
    }
  } catch (error) {
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
