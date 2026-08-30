import Datastore from '@seald-io/nedb';

const db = new Datastore();

export async function addinterval(modelid, channel, intervalID) {
    const model = {
        id: modelid,
        channel: channel,
        intervalID: intervalID
    }
    try {
        const insertmodel = await db.insertAsync(model)
    } catch (error) {
        console.log(error);
    }
}

export async function deleteinterval(modelid, channel) {
    try {
        const deletemodel = await db.removeAsync({id: modelid, channel: channel}, {});
        return deletemodel;
    } catch (error) {
        console.log(error);
    }
}

export async function getinterval(modelid, channel) {
    const stuff = await db.findAsync({id: modelid, channel: channel});
    return stuff[0]['intervalID'];
}