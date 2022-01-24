import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormArray, FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { GROUP } from 'src/app/models/group.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateGroupService } from './create-group.service';

@Component({
	selector: 'app-create-group',
	templateUrl: './create-group.component.html',
	styleUrls: ['./create-group.component.scss']
})
export class CreateGroupComponent implements OnInit {
	@Input() selectedGroup!: GROUP;
	@Input() editMode: boolean = false;
	@Output() closeModal = new EventEmitter<boolean>();
	disableEndpoint: boolean = true;
	loading: boolean = false;
	hashes!: string[];
	createGroupForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		strategy: this.formBuilder.group({
			interval_seconds: ['', Validators.required],
			limit: ['', Validators.required]
		}),
		signature: this.formBuilder.group({
			header: ['', Validators.required],
			hash: ['', Validators.required]
		}),
		disable_endpoint: [false, Validators.required]
	});
	constructor(private formBuilder: FormBuilder, private createGroupService: CreateGroupService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getHashes();
		if (this.editMode) {
			this.editGroup();
		}
	}
	getStrategyControls() {
		return (this.createGroupForm.get('strategy') as FormArray).controls;
	}
	getSignatureControls() {
		return (this.createGroupForm.get('signature') as FormArray).controls;
	}
	async createGroup() {
		
		if (this.createGroupForm.invalid) {
			(<any>Object).values(this.createGroupForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		const orgId = localStorage.getItem('orgId');
		this.loading = true;
		const requestOptions = {
			orgId: `org_id=${orgId}`,
			groupId: this.selectedGroup?.id
		};
		try {
			let response;
			if (this.editMode) {
				response = await this.createGroupService.editGroup(this.createGroupForm.value, requestOptions);
			} else {
				response = await this.createGroupService.createGroup(this.createGroupForm.value, requestOptions);
			}
			this.closeModal.emit(true);
			this.createGroupForm.reset();
			this.generalService.showNotification({ message: response.message });
			this.loading = false;
		} catch (error) {
			this.loading = false;
		}
	}
	updateForm() {
		this.createGroupForm.patchValue({
			disable_endpoint: this.disableEndpoint
		});
	}
	editGroup() {
		this.createGroupForm.patchValue({
			name: this.selectedGroup?.name,
			strategy: {
				interval_seconds: this.selectedGroup?.config?.Strategy?.default?.intervalSeconds,
				limit: this.selectedGroup?.config?.Strategy?.default?.retryLimit
			},
			signature: {
				header: this.selectedGroup?.config?.Signature?.header,
				hash: this.selectedGroup?.config?.Signature?.hash
			}
		});
		this.disableEndpoint = this.selectedGroup?.config?.DisableEndpoint;
	}

	async getHashes() {
		try {
			const response = await this.createGroupService.getHashes();
			this.hashes = response.data.hashes;
		} catch {
			this.generalService.showNotification({ message: 'Unable to retrieve hashes' });
		}
	}
	closeCreateGroupModal() {
		this.closeModal.emit();
	}
	onlyNumber(event: KeyboardEvent): boolean {
		return this.generalService.onlyNumber(event);
	}
}