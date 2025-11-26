// Region mapping from assets/regions.json
// Maps area ID (e.g., "JP13") to region name (e.g., "東京")

export const regionsData = {
  "hokkaido-tohoku": [
    { id: "JP1", name: "北海道" },
    { id: "JP2", name: "青森" },
    { id: "JP3", name: "岩手" },
    { id: "JP4", name: "宮城" },
    { id: "JP5", name: "秋田" },
    { id: "JP6", name: "山形" },
    { id: "JP7", name: "福島" },
  ],
  "kanto": [
    { id: "JP8", name: "茨城" },
    { id: "JP9", name: "栃木" },
    { id: "JP10", name: "群馬" },
    { id: "JP11", name: "埼玉" },
    { id: "JP12", name: "千葉" },
    { id: "JP13", name: "東京" },
    { id: "JP14", name: "神奈川" },
  ],
  "hokuriku-koushinetsu": [
    { id: "JP15", name: "新潟" },
    { id: "JP19", name: "山梨" },
    { id: "JP20", name: "長野" },
    { id: "JP17", name: "石川" },
    { id: "JP16", name: "富山" },
    { id: "JP18", name: "福井" },
  ],
  "chubu": [
    { id: "JP23", name: "愛知" },
    { id: "JP21", name: "岐阜" },
    { id: "JP22", name: "静岡" },
    { id: "JP24", name: "三重" },
  ],
  "kinki": [
    { id: "JP27", name: "大阪" },
    { id: "JP28", name: "兵庫" },
    { id: "JP26", name: "京都" },
    { id: "JP25", name: "滋賀" },
    { id: "JP29", name: "奈良" },
    { id: "JP30", name: "和歌山" },
  ],
  "chugoku-shikoku": [
    { id: "JP33", name: "岡山" },
    { id: "JP34", name: "広島" },
    { id: "JP31", name: "鳥取" },
    { id: "JP32", name: "島根" },
    { id: "JP35", name: "山口" },
    { id: "JP37", name: "香川" },
    { id: "JP36", name: "徳島" },
    { id: "JP38", name: "愛媛" },
    { id: "JP39", name: "高知" },
  ],
  "kyushu": [
    { id: "JP40", name: "福岡" },
    { id: "JP41", name: "佐賀" },
    { id: "JP42", name: "長崎" },
    { id: "JP43", name: "熊本" },
    { id: "JP44", name: "大分" },
    { id: "JP45", name: "宮崎" },
    { id: "JP46", name: "鹿児島" },
    { id: "JP47", name: "沖縄" },
  ],
};

// Create a flat map for quick lookup
const areaIdToNameMap = new Map<string, string>();

Object.values(regionsData).forEach((regions) => {
  regions.forEach((region) => {
    areaIdToNameMap.set(region.id, region.name);
  });
});

/**
 * Gets the region name for a given area ID
 * @param areaId - The area ID (e.g., "JP13")
 * @returns The region name (e.g., "東京") or undefined if not found
 */
export function getAreaName(areaId: string | null | undefined): string | undefined {
  if (!areaId) {
    return undefined;
  }
  return areaIdToNameMap.get(areaId);
}

