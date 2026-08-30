import sqlite3 from "sqlite3";

const db = new sqlite3.Database('mfcbot.db');

process.on('SIGINT', () => {
  db.close((err) => {
    if (err) console.error(err.message);
    process.exit(err ? 1 : 0);
  });
});

// FIXED: Added CURRENT_TIMESTAMP logic or parameter alignment
export async function addmodel(modelid, modelname, message, channel, update, events) {
  return new Promise((resolve, reject) => {
    db.run(
      "INSERT INTO models(modelid, modelname, message, time, channel, updatetime, events) VALUES (?,?,?,0,?,?,?)", 
      [modelid, modelname, message, channel, update, events], 
      function (err) {
        if (err) return reject(err); 
        resolve(this.changes); 
      }
    );
  });
}

// FIXED: Changed table name from 'users' to 'models'
export async function deletemodel(modelid, channel) {
  return new Promise((resolve, reject) => {
    db.run(
      `DELETE FROM models WHERE modelid = ? AND channel = ?`,
      [modelid, channel],
      function (err) {
        if (err) return reject(err); 
        resolve(this.changes); 
      }
    );
  });
}

// FIXED: Reordered parameters array to match the order of '?' in the SQL query
export async function updatemodel(modelid, modelname, message, channel, update, events) {
  return new Promise((resolve, reject) => {
    db.run(
      "UPDATE models SET modelname = ?, message = ?, updatetime = ?, events = ? WHERE modelid = ? AND channel = ?", 
      [modelname, message, update, events, modelid, channel], 
      function (err) {
        if (err) return reject(err); 
        resolve(this.changes); 
      }
    );
  });
}

// FIXED: Wrapped in Promise so it doesn't return 'undefined'
export async function getmodels() {
  return new Promise((resolve, reject) => {
    db.all("SELECT * FROM models", [], (err, rows) => {
      if (err) return reject(err);
      resolve(rows);
    });
  });
}

// FIXED: Wrapped in Promise so it doesn't return 'undefined'
export async function getmodel(modelid, channel) {
  return new Promise((resolve, reject) => {
    db.get("SELECT * FROM models WHERE modelid = ? AND channel = ?", [modelid, channel], (err, row) => {
      if (err) return reject(err);
      resolve(row);
    });
  });
}

// FIXED: Wrapped in Promise and returns the direct string value instead of the raw row object
export async function gettopic(modelid, channel) {
  return new Promise((resolve, reject) => {
    db.get("SELECT topic FROM models WHERE modelid = ? AND channel = ?", [modelid, channel], (err, row) => {
      if (err) return reject(err);
      resolve(row ? row.topic : null);
    });
  });
}

// FIXED: Wrapped in Promise and returns the direct timestamp value instead of the raw row object
export async function gettime(modelid, channel) {
  return new Promise((resolve, reject) => {
    db.get("SELECT time FROM models WHERE modelid = ? AND channel = ?", [modelid, channel], (err, row) => {
      if (err) return reject(err);
      resolve(row ? row.time : null);
    });
  });
}

export async function updatetopic(modelid, topic, channel) {
  return new Promise((resolve, reject) => {
    db.run("UPDATE models SET topic = ? WHERE modelid = ? AND channel = ?", [topic, modelid, channel], function (err) {
        if (err) return reject(err); 
        resolve(this.changes); 
      }
    );
  });
}

export async function updatetime(modelid, time, channel) {
  return new Promise((resolve, reject) => {
    db.run("UPDATE models SET time = ? WHERE modelid = ? AND channel = ?", [time, modelid, channel], function (err) {
        if (err) return reject(err); 
        resolve(this.changes); 
      }
    );
  });
}


/*export async function updatemodel(modelid, modelname, message, channel, update, events) {
    const model = {
        id: modelid,
        modelname: modelname,
        message: message,
        time: false,
        channel: channel,
        update: update,
        topic: false,
        events: events
    }
    console.log(model);
    try {
        const insertmodel = await db.updateAsync({id: modelid, channel: channel} , model);
        return insertmodel;
    } catch (error) {
        console.log(error);
    }
}
*/