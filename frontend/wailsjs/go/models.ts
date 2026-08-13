export namespace config {
	
	export class KeyLockConfig {
	    enabled: boolean;
	    keys: string[];
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new KeyLockConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.keys = source["keys"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class ActionStep {
	    key: string;
	    hold_ms: number;
	    delay_ms?: number;
	    hold_down?: boolean;
	    release?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.hold_ms = source["hold_ms"];
	        this.delay_ms = source["delay_ms"];
	        this.hold_down = source["hold_down"];
	        this.release = source["release"];
	    }
	}
	export class Action {
	    id: string;
	    name: string;
	    description: string;
	    enabled: boolean;
	    reward_title: string;
	    reward_cost: number;
	    reward_id?: string;
	    reward_color?: string;
	    key: string;
	    hold_ms: number;
	    repeat?: number;
	    repeat_delay_ms?: number;
	    steps?: ActionStep[];
	    key_lock: KeyLockConfig;
	    cooldown_ms: number;
	    twitch_cooldown_sec?: number;
	    category: string;
	    sc_actionmap?: string;
	    sc_action?: string;
	    custom?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.reward_title = source["reward_title"];
	        this.reward_cost = source["reward_cost"];
	        this.reward_id = source["reward_id"];
	        this.reward_color = source["reward_color"];
	        this.key = source["key"];
	        this.hold_ms = source["hold_ms"];
	        this.repeat = source["repeat"];
	        this.repeat_delay_ms = source["repeat_delay_ms"];
	        this.steps = this.convertValues(source["steps"], ActionStep);
	        this.key_lock = this.convertValues(source["key_lock"], KeyLockConfig);
	        this.cooldown_ms = source["cooldown_ms"];
	        this.twitch_cooldown_sec = source["twitch_cooldown_sec"];
	        this.category = source["category"];
	        this.sc_actionmap = source["sc_actionmap"];
	        this.sc_action = source["sc_action"];
	        this.custom = source["custom"];
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
	
	export class TwitchConfig {
	    client_id: string;
	    client_secret?: string;
	    access_token?: string;
	    refresh_token?: string;
	    // Go type: time
	    expires_at?: any;
	    channel_name: string;
	    broadcaster_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new TwitchConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client_id = source["client_id"];
	        this.client_secret = source["client_secret"];
	        this.access_token = source["access_token"];
	        this.refresh_token = source["refresh_token"];
	        this.expires_at = this.convertValues(source["expires_at"], null);
	        this.channel_name = source["channel_name"];
	        this.broadcaster_id = source["broadcaster_id"];
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
	export class Config {
	    twitch: TwitchConfig;
	    actions: Action[];
	    target_window: string;
	    global_enable: boolean;
	    language: string;
	    input_mode?: string;
	    sc_dir?: string;
	    sc_channel?: string;
	    migrated_real_keys?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.twitch = this.convertValues(source["twitch"], TwitchConfig);
	        this.actions = this.convertValues(source["actions"], Action);
	        this.target_window = source["target_window"];
	        this.global_enable = source["global_enable"];
	        this.language = source["language"];
	        this.input_mode = source["input_mode"];
	        this.sc_dir = source["sc_dir"];
	        this.sc_channel = source["sc_channel"];
	        this.migrated_real_keys = source["migrated_real_keys"];
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
	

}

export namespace main {
	
	export class BindStatus {
	    action_id: string;
	    name: string;
	    enabled: boolean;
	    sc_action: string;
	    key: string;
	    effective: string;
	    source: string;
	    mismatch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BindStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action_id = source["action_id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.sc_action = source["sc_action"];
	        this.key = source["key"];
	        this.effective = source["effective"];
	        this.source = source["source"];
	        this.mismatch = source["mismatch"];
	    }
	}
	export class DeviceAuthInfo {
	    user_code: string;
	    verification_uri: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceAuthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_code = source["user_code"];
	        this.verification_uri = source["verification_uri"];
	    }
	}
	export class TwitchApp {
	    client_id: string;
	    has_secret: boolean;
	    is_default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TwitchApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.client_id = source["client_id"];
	        this.has_secret = source["has_secret"];
	        this.is_default = source["is_default"];
	    }
	}

}

export namespace starcitizen {
	
	export class Install {
	    channel: string;
	    dir: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Install(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.dir = source["dir"];
	        this.source = source["source"];
	    }
	}

}

