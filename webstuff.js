import fetch from 'node-fetch';

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

export async function getmodelid(modelname) {
    ////console.log("function");
    var url = "https://share.myfreecams.com/" + modelname;
    try {
        const response = await fetch(url);
        const body = await response.text();
        let result = body.match(/tracking\.php\?model_id=(\d+)&/);
        return result[1];
    } catch (error) {
        console.log(error);
        return "Lookup failed";
    }
    ////console.log(result[1]);
}

export async function getstatus(modelid) {

    //console.log("Entered getstatus" + modelid);

    var url = "https://api-edge.myfreecams.com/usernameLookup/" + modelid;
    const modelstatus = {
        status: 1,
        topic: ""
    }
    //console.log(modelstatus);
    try {
        const response = await fetch(url);
        const body = await response.json();
        //console.log("Try");
        if (body['result']['success'] === 0) {
            //console.log("offline");
            //console.log(body['result']['message'])
            //console.log(modelstatus);
            return modelstatus;
        }
        
        if (0 in body['result']['user']['sessions']) {
            //console.log("Online");
            let status = body['result']['user']['sessions'][0];
            //console.log("modelstatus");
            modelstatus['status'] = status['vstate'];
            modelstatus['topic'] = status['room_topic'];
            //console.log(modelstatus);
            return modelstatus;
        } else {
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