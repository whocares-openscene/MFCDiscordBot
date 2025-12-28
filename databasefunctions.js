import Datastore from '@seald-io/nedb';

const db = new Datastore({ filename: 'mfcbot.db', autoload: true });

export async function addmodel(modelid, modelname, message, channel, update) {
    const model = {
        id: modelid,
        modelname: modelname,
        message: message,
        time: false,
        channel: channel,
        update: update,
        topic: false
    }
    try {
        const insertmodel = await db.insertAsync(model)
    } catch (error) {
        console.log(error);
    }
}

export async function deletemodel(modelid, channel) {
    try {
        const deletemodel = await db.removeAsync({id: modelid, channel: channel}, {});
        return deletemodel;
    } catch (error) {
        console.log(error);
    }
}

export async function updatemodel(modelid, modelname, message, channel, update) {
    const model = {
        id: modelid,
        modelname: modelname,
        message: message,
        time: false,
        channel: channel,
        update: update,
        topic: false
    }
    try {
        const insertmodel = await db.updateAsync({id: modelid, channel: channel} , model);
        return insertmodel;
    } catch (error) {

    }
}

export async function getmodels() {
    return await db.findAsync({})
}

export async function getmodel(modelid, channel) {
    return await db.findAsync({id: modelid, channel: channel});
}

export async function gettopic(modelid, channel) {
    const model = await db.findAsync({id: modelid, channel: channel});
    return model['topic'];
}

export async function gettime(modelid, channel) {
    const model = await db.findAsync({id: modelid, channel: channel});
    return model['time'];
}

export async function updatetopic(modelid, topic, channel) {
    await db.updateAsync({ id: modelid, channel: channel }, { $set: { topic: topic } }, { multi: false })
}

export async function updatetime(modelid, time, channel) {
    await db.updateAsync({ id: modelid, channel: channel }, { $set: { time: time } }, { multi: false })
}

