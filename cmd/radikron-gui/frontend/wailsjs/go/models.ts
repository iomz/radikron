export namespace config {
	
	export class Config {
	    AreaID: string;
	    ExtraStations: string[];
	    IgnoreStations: string[];
	    FileFormat: string;
	    MinimumOutputSize: number;
	    DownloadDir: string;
	    Rules: radikron.Rule[];
	    MaxDownloadingConcurrency: number;
	    MaxEncodingConcurrency: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.AreaID = source["AreaID"];
	        this.ExtraStations = source["ExtraStations"];
	        this.IgnoreStations = source["IgnoreStations"];
	        this.FileFormat = source["FileFormat"];
	        this.MinimumOutputSize = source["MinimumOutputSize"];
	        this.DownloadDir = source["DownloadDir"];
	        this.Rules = this.convertValues(source["Rules"], radikron.Rule);
	        this.MaxDownloadingConcurrency = source["MaxDownloadingConcurrency"];
	        this.MaxEncodingConcurrency = source["MaxEncodingConcurrency"];
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

export namespace radikron {
	
	export class ProgGenre {
	    Personality: string;
	    Program: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgGenre(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Personality = source["Personality"];
	        this.Program = source["Program"];
	    }
	}
	export class Prog {
	    ID: string;
	    StationID: string;
	    Ft: string;
	    To: string;
	    Title: string;
	    Desc: string;
	    Info: string;
	    Pfm: string;
	    Tags: string[];
	    Genre: ProgGenre;
	    M3U8: string;
	    RuleName: string;
	    RuleFolder: string;
	    IsManualInjection: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Prog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StationID = source["StationID"];
	        this.Ft = source["Ft"];
	        this.To = source["To"];
	        this.Title = source["Title"];
	        this.Desc = source["Desc"];
	        this.Info = source["Info"];
	        this.Pfm = source["Pfm"];
	        this.Tags = source["Tags"];
	        this.Genre = this.convertValues(source["Genre"], ProgGenre);
	        this.M3U8 = source["M3U8"];
	        this.RuleName = source["RuleName"];
	        this.RuleFolder = source["RuleFolder"];
	        this.IsManualInjection = source["IsManualInjection"];
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
	
	export class Rule {
	    Name: string;
	    Title: string;
	    DoW: string[];
	    Keyword: string;
	    Pfm: string;
	    StationID: string;
	    Window: string;
	    Folder: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Title = source["Title"];
	        this.DoW = source["DoW"];
	        this.Keyword = source["Keyword"];
	        this.Pfm = source["Pfm"];
	        this.StationID = source["StationID"];
	        this.Window = source["Window"];
	        this.Folder = source["Folder"];
	    }
	}

}

