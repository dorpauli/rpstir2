package parsevalidate

import (
	"net/url"
	"time"

	belogs "github.com/astaxie/beego/logs"
	jsonutil "github.com/cpusoft/goutil/jsonutil"

	. "model"
	"parsevalidate/openssl"
)

//Try to store the error in statemode instead of returning err
func ParseEeCertModel(certFile string, fileByte []byte, start int, end int) (eeCertModel EeCertModel, err error) {
	eeCertModel = EeCertModel{}
	eeCertModel.EeCertStartByte = uint64(start)
	eeCertModel.EeCertEndByte = uint64(end)
	err = openssl.ParseEeCertModelByX509(fileByte, &eeCertModel)
	if err != nil {
		belogs.Error("ParseEeCertModel():ParseEeCertModelByX509 err:", err)
		return eeCertModel, err
	}

	results, err := openssl.GetResultsByOpensslX509(certFile)
	if err != nil {
		belogs.Error("ParseEeCertModel(): GetResultsByOpensslX509: err: ", err, ": "+certFile)
		return eeCertModel, err
	}
	belogs.Debug("ParseEeCertModel(): GetResultsByOpensslX509 len(results):", len(results))

	//keyusage ,critical
	eeCertModel.KeyUsageModel.Critical, eeCertModel.KeyUsageModel.KeyUsageValue, err = openssl.ParseKeyUsageModelByOpensslResults(results)
	if err != nil {
		belogs.Error("ParseEeCertModel(): ParseKeyUsageModelByOpensslResults: err: ", err, ": "+certFile)
		return eeCertModel, err
	}

	// AIA SIA
	_, eeCertModel.SiaModel, err = openssl.ParseAiaModelSiaModelByOpensslResults(results)
	if err != nil {
		belogs.Error("ParseEeCertModel(): ParseAiaModelSiaModelByOpensslResults: err: ", err, ": "+certFile)
		return eeCertModel, err
	}

	belogs.Debug("ParseEeCertModel(): eeCertModel:", jsonutil.MarshalJson(eeCertModel))

	return eeCertModel, nil
}

// https://datatracker.ietf.org/doc/rfc6488/?include_text=1  Signed Object Template for the Resource Public Key Infrastructure (RPKI)
// roa_validate.c P509
func ValidateEeCertModel(stateModel *StateModel, eeCertModel *EeCertModel) error {
	if eeCertModel.Version != 3 {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "Version number of EE is not 3",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if eeCertModel.DigestAlgorithm != "SHA256-RSA" {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "Digest Algorithm of EE is not sha256WithRSAEncryption",
			Detail: "Digest algorithm is" + eeCertModel.DigestAlgorithm}
		stateModel.AddError(&stateMsg)
	}
	now := time.Now()
	if eeCertModel.NotBefore.IsZero() {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "NotBefore of EE is emtpy",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if eeCertModel.NotAfter.IsZero() {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "NotAfter of EE is empy",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	//thisUpdate precedes nextUpdate.
	if eeCertModel.NotBefore.After(now) {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "NotBefore of EE is later than the current time",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if eeCertModel.NotAfter.Before(now) {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "NotAfter of EE is earlier than the current time",
			Detail: ""}
		stateModel.AddWarning(&stateMsg)
	}
	if eeCertModel.NotAfter.Before(eeCertModel.NotBefore) {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "NotAfter of EE is earlier than NotBefore of EE",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}

	// check basic_constraints    myssl.c P2100
	//Basic constraints in EE cert"
	if eeCertModel.IsCa {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "IsCA of EE is not true",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if eeCertModel.BasicConstraintsValid {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "BasicConstraintsValid of EE is not true",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}

	// rescert_key_usage_chk myssl.c P2359  TODO

	// rfc6487#section-4.8.5   rescert_extended_key_usage_chk, myssl.c P2427
	if len(eeCertModel.ExtKeyUsages) > 0 {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "ExKeyUsage of EE is not empty",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}

	// check sia
	// rescert_sia_chk myssl.c P2813    RFC6487 4.8.8.2. SIA for EE Certificates
	if eeCertModel.SiaModel.Critical {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "SIA of EE is critical",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if len(eeCertModel.SiaModel.SignedObject) == 0 {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "SignedObject of EE is empty",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	} else {
		u, err := url.Parse(eeCertModel.SiaModel.SignedObject)
		if err != nil {
			stateMsg := StateMsg{Stage: "parsevalidate",
				Fail:   "SignedObject of EE is not a legal URL address",
				Detail: err.Error()}
			stateModel.AddError(&stateMsg)
		}
		if u.Scheme != "rsync" {
			stateMsg := StateMsg{Stage: "parsevalidate",
				Fail:   "SignedObject of EE is not an Rsync protocol",
				Detail: ""}
			stateModel.AddError(&stateMsg)
		}
	}
	if len(eeCertModel.SiaModel.CaRepository) > 0 {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "CA Repository of EE is not empty",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if len(eeCertModel.SiaModel.RpkiManifest) > 0 {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "RpkiMainfest of EE is not empty",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	return nil
}

// https://datatracker.ietf.org/doc/rfc6488/?include_text=1  Signed Object Template for the Resource Public Key Infrastructure (RPKI)
func ValidateSignerInfoModel(stateModel *StateModel, signerInfoModel *SignerInfoModel) error {

	if signerInfoModel.DigestAlgorithm != "sha256" {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "Digest Algorithm of SignerInfo is not sha256",
			Detail: "Digest Algorithm of SignerInfo is " + signerInfoModel.DigestAlgorithm}
		stateModel.AddError(&stateMsg)
	}
	now := time.Now()
	if signerInfoModel.SiningTime.IsZero() {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "SiningTime of SignerInfo is empty",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	if signerInfoModel.SiningTime.After(now) {
		stateMsg := StateMsg{Stage: "parsevalidate",
			Fail:   "SiningTime of SignerInfo is later than the current time",
			Detail: ""}
		stateModel.AddError(&stateMsg)
	}
	return nil

}