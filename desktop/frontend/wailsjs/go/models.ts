export namespace main {
	
	export class ActiveCall {
	    device: string;
	    state: string;
	    direction: string;
	    number: string;
	    name: string;
	    photo: string;
	    since: number;
	
	    static createFrom(source: any = {}) {
	        return new ActiveCall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.state = source["state"];
	        this.direction = source["direction"];
	        this.number = source["number"];
	        this.name = source["name"];
	        this.photo = source["photo"];
	        this.since = source["since"];
	    }
	}
	export class CallLogEntry {
	    number: string;
	    name: string;
	    type: string;
	    date: number;
	    duration: number;
	    sim: number;
	
	    static createFrom(source: any = {}) {
	        return new CallLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.date = source["date"];
	        this.duration = source["duration"];
	        this.sim = source["sim"];
	    }
	}
	export class ClipboardEntry {
	    text: string;
	    from: string;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new ClipboardEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.from = source["from"];
	        this.time = source["time"];
	    }
	}
	export class ClipboardHelper {
	    running: boolean;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ClipboardHelper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.installed = source["installed"];
	    }
	}
	export class Contact {
	    id: string;
	    name: string;
	    numbers: string[];
	    photo: string;
	
	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.numbers = source["numbers"];
	        this.photo = source["photo"];
	    }
	}
	export class MediaPreview {
	    url: string;
	    video: boolean;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new MediaPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.video = source["video"];
	        this.duration = source["duration"];
	    }
	}
	export class PairingInfo {
	    deviceId: string;
	    code: string;
	    relay: string;
	
	    static createFrom(source: any = {}) {
	        return new PairingInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.code = source["code"];
	        this.relay = source["relay"];
	    }
	}
	export class PhoneNotification {
	    id: string;
	    device: string;
	    app: string;
	    appId: string;
	    title: string;
	    text: string;
	    time: number;
	    actions: string[];
	    replyable: boolean;
	    icon?: string;
	
	    static createFrom(source: any = {}) {
	        return new PhoneNotification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.device = source["device"];
	        this.app = source["app"];
	        this.appId = source["appId"];
	        this.title = source["title"];
	        this.text = source["text"];
	        this.time = source["time"];
	        this.actions = source["actions"];
	        this.replyable = source["replyable"];
	        this.icon = source["icon"];
	    }
	}
	export class RelayStatus {
	    configured: boolean;
	    status: string;
	    detail: string;
	    server: string;
	    code: string;
	    peers: number;
	
	    static createFrom(source: any = {}) {
	        return new RelayStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.server = source["server"];
	        this.code = source["code"];
	        this.peers = source["peers"];
	    }
	}
	export class SIM {
	    id: number;
	    slot: number;
	    name: string;
	    carrier: string;
	
	    static createFrom(source: any = {}) {
	        return new SIM(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.slot = source["slot"];
	        this.name = source["name"];
	        this.carrier = source["carrier"];
	    }
	}
	export class SMSMessage {
	    id: number;
	    threadId: number;
	    address: string;
	    body: string;
	    date: number;
	    type: number;
	    read: boolean;
	    name: string;
	    unread?: number;
	    subscription: number;
	
	    static createFrom(source: any = {}) {
	        return new SMSMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.threadId = source["threadId"];
	        this.address = source["address"];
	        this.body = source["body"];
	        this.date = source["date"];
	        this.type = source["type"];
	        this.read = source["read"];
	        this.name = source["name"];
	        this.unread = source["unread"];
	        this.subscription = source["subscription"];
	    }
	}
	export class UpdateInfo {
	    isNewer: boolean;
	    latestVersion: string;
	    downloadUrl: string;
	    releaseNotes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isNewer = source["isNewer"];
	        this.latestVersion = source["latestVersion"];
	        this.downloadUrl = source["downloadUrl"];
	        this.releaseNotes = source["releaseNotes"];
	    }
	}

}

export namespace models {
	
	export class HistoryFile {
	    name: string;
	    size: number;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.path = source["path"];
	    }
	}
	export class HistoryEntry {
	    id: string;
	    partnerName: string;
	    files: HistoryFile[];
	    totalSize: number;
	    direction: string;
	    // Go type: time
	    timestamp: any;
	    status: string;
	    errorMessage: string;
	    durationMillis: number;
	    kind?: string;
	    text?: string;
	    copied?: boolean;
	    route?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.partnerName = source["partnerName"];
	        this.files = this.convertValues(source["files"], HistoryFile);
	        this.totalSize = source["totalSize"];
	        this.direction = source["direction"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	        this.durationMillis = source["durationMillis"];
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.copied = source["copied"];
	        this.route = source["route"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Peer {
	    id: string;
	    deviceName: string;
	    ipAddress: string;
	    port: number;
	    os: string;
	    via: string;
	    deviceId: string;
	    paired: boolean;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.deviceName = source["deviceName"];
	        this.ipAddress = source["ipAddress"];
	        this.port = source["port"];
	        this.os = source["os"];
	        this.via = source["via"];
	        this.deviceId = source["deviceId"];
	        this.paired = source["paired"];
	        this.path = source["path"];
	    }
	}

}

