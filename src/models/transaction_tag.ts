export class TransactionTag implements TransactionTagInfoResponse {
    public id: string;
    public name: string;
    public groupId: string;
    public displayOrder: number;
    public hidden: boolean;
    public parentId = '0';

    private constructor(id: string, name: string, groupId: string, displayOrder: number, hidden: boolean) {
        this.id = id;
        this.name = name;
        this.groupId = groupId;
        this.displayOrder = displayOrder;
        this.hidden = hidden;
    }

    public toCreateRequest(): TransactionTagCreateRequest {
        return {
            name: this.name,
            parentId: this.parentId,
            groupId: this.groupId
        };
    }

    public toModifyRequest(): TransactionTagModifyRequest {
        return {
            id: this.id,
            parentId: this.parentId,
            groupId: this.groupId,
            name: this.name
        };
    }

    public clone(): TransactionTag {
        const tag = new TransactionTag(this.id, this.name, this.groupId, this.displayOrder, this.hidden);
        tag.parentId = this.parentId;
        return tag;
    }

    public static of(tagResponse: TransactionTagInfoResponse): TransactionTag {
        const tag = new TransactionTag(tagResponse.id, tagResponse.name, tagResponse.groupId, tagResponse.displayOrder, tagResponse.hidden);
        tag.parentId = tagResponse.parentId || '0';
        return tag;
    }

    public static ofMulti(tagResponses: TransactionTagInfoResponse[]): TransactionTag[] {
        const tags: TransactionTag[] = [];

        for (const tagResponse of tagResponses) {
            tags.push(TransactionTag.of(tagResponse));
        }

        return tags;
    }

    public static createNewTag(name?: string, groupId?: string): TransactionTag {
        return new TransactionTag('', name || '', groupId || '0', 0, false);
    }
}

export interface TransactionTagCreateRequest {
    readonly parentId?: string;
    readonly groupId: string;
    readonly name: string;
}

export interface TransactionTagCreateBatchRequest {
    readonly tags: TransactionTagCreateRequest[];
    readonly groupId: string;
    readonly skipExists: boolean;
}

export interface TransactionTagModifyRequest {
    readonly parentId?: string;
    readonly id: string;
    readonly groupId: string;
    readonly name: string;
}

export interface TransactionTagHideRequest {
    readonly id: string;
    readonly hidden: boolean;
}

export interface TransactionTagMoveRequest {
    readonly newDisplayOrders: TransactionTagNewDisplayOrderRequest[];
}

export interface TransactionTagNewDisplayOrderRequest {
    readonly id: string;
    readonly displayOrder: number;
}

export interface TransactionTagDeleteRequest {
    readonly id: string;
}

export interface TransactionTagInfoResponse {
    readonly parentId?: string;
    readonly id: string;
    readonly name: string;
    readonly groupId: string;
    readonly displayOrder: number;
    readonly hidden: boolean;
}
