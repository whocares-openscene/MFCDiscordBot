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
    //console.log("function");
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
    //console.log(result[1]);
}
